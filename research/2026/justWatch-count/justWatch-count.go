package main

import (
   "bytes"
   _ "embed"
   "encoding/json"
   "flag"
   "fmt"
   "io"
   "log"
   "net/http"
   "os"
   "strings"
)

//go:embed GetUrlMetadata.graphql
var metadataQuery string

//go:embed GetProviderTop10TitlesFallback.graphql
var titlesQuery string

// InputRequest models the expected structure of the input JSON file
type InputRequest struct {
   Path string `json:"path"`
}

// Provider models the JustWatch REST API response, using the slug
type Provider struct {
   ShortName string `json:"short_name"`
   ClearName string `json:"clear_name"`
   Slug      string `json:"slug"`
}

// Global cache to prevent re-downloading the heavy provider list for the same locale
var providerCache = make(map[string][]Provider)

func main() {
   fileFlag := flag.String("file", "", "Path to JSON file containing an array of paths (required)")
   flag.Parse()

   if *fileFlag == "" {
      flag.Usage()
      log.Fatal("Error: the -file flag is required.")
   }

   fileBytes, err := os.ReadFile(*fileFlag)
   if err != nil {
      log.Fatalf("Failed to read file '%s': %v", *fileFlag, err)
   }

   var paths []string
   if err := json.Unmarshal(fileBytes, &paths); err != nil {
      log.Fatalf("Failed to parse JSON data: %v", err)
   }

   for _, path := range paths {
      path = strings.TrimSpace(path)
      if path == "" {
         log.Println("Skipping empty path entry")
         continue
      }

      // 1. Get Locale & Country from Path
      locale, err := fetchLocaleFromPath(path)
      if err != nil {
         log.Fatalf("[%s] Failed to get metadata: %v\n", path, err)
      }

      parts := strings.Split(locale, "_")
      if len(parts) != 2 {
         log.Fatalf("[%s] Invalid locale format received: %s\n", path, locale)
      }
      country := parts[1]

      // 2. Resolve the Package Code (shortName) using the Locale & Path
      pkgCode, providerName, err := resolvePackageFromPath(path, locale)
      if err != nil {
         log.Fatalf("[%s] Failed to resolve package code: %v\n", path, err)
      }

      // 3. Fetch the Total Count
      totalCount, err := fetchTotalCount(pkgCode, country)
      if err != nil {
         log.Fatalf("[%s] Failed to fetch total count: %v\n", path, err)
      }

      fmt.Printf("Path: %-45s | Country: %s | Package: %-4s | Provider: %-20s | Total: %d\n",
         path, country, pkgCode, providerName, totalCount)
   }
}

// fetchLocaleFromPath runs GetUrlMetadata to find the JustWatch locale (e.g. "en_US")
func fetchLocaleFromPath(path string) (string, error) {
   payload := map[string]interface{}{
      "operationName": "GetUrlMetadata",
      "variables": map[string]interface{}{
         "fullPath": path,
         "site":     "www",
      },
      "query": metadataQuery,
   }

   body, err := executeGraphQL(payload)
   if err != nil {
      return "", err
   }

   var resp struct {
      Data struct {
         UrlV2 struct {
            Locale string `json:"locale"`
         } `json:"urlV2"`
      } `json:"data"`
   }

   if err := json.Unmarshal(body, &resp); err != nil {
      return "", err
   }
   if resp.Data.UrlV2.Locale == "" {
      return "", fmt.Errorf("API returned empty locale for path")
   }

   return resp.Data.UrlV2.Locale, nil
}

// resolvePackageFromPath fetches the provider list (using cache) and matches the slug
func resolvePackageFromPath(path, locale string) (string, string, error) {
   // Extract the slug from the end of the URL path (e.g. "/us/provider/hbo-max" -> "hbo-max")
   cleanPath := strings.TrimRight(path, "/")
   segments := strings.Split(cleanPath, "/")
   urlSlug := segments[len(segments)-1]

   // Check the in-memory cache first to avoid slamming the network
   providers, cached := providerCache[locale]

   if !cached {
      restURL := fmt.Sprintf("https://apis.justwatch.com/content/providers/locale/%s", locale)

      req, err := http.NewRequest("GET", restURL, nil)
      if err != nil {
         return "", "", err
      }
      req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")

      client := &http.Client{}
      resp, err := client.Do(req)
      if err != nil {
         return "", "", err
      }
      defer resp.Body.Close()

      if resp.StatusCode != 200 {
         return "", "", fmt.Errorf("REST API returned status %d", resp.StatusCode)
      }

      if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
         return "", "", err
      }

      // Save to cache for the next time this locale is requested
      providerCache[locale] = providers
   }

   // Exact match using the API's slug
   for _, p := range providers {
      if p.Slug == urlSlug {
         return p.ShortName, p.ClearName, nil
      }
   }

   return "", "", fmt.Errorf("could not find provider with slug '%s' in locale '%s'", urlSlug, locale)
}

// fetchTotalCount runs the GetProviderTop10TitlesFallback query to get the catalog size
func fetchTotalCount(pkg, country string) (int, error) {
   variables := map[string]interface{}{
      "country":             country,
      "first":               0, // 0 is fine since we aren't fetching any edges
      "popularTitlesSortBy": "TRENDING",
      "popularTitlesFilter": map[string]interface{}{
         "packages": []string{pkg},
         "tomatoMeter": map[string]int{
            "min": 60,
         },
      },
   }

   payload := map[string]interface{}{
      "operationName": "GetProviderTop10TitlesFallback",
      "variables":     variables,
      "query":         titlesQuery,
   }

   body, err := executeGraphQL(payload)
   if err != nil {
      return 0, err
   }

   var resp struct {
      Errors []struct {
         Message string `json:"message"`
      } `json:"errors"`
      Data *struct {
         PopularTitles struct {
            TotalCount int `json:"totalCount"`
         } `json:"popularTitles"`
      } `json:"data"`
   }

   if err := json.Unmarshal(body, &resp); err != nil {
      return 0, err
   }
   if len(resp.Errors) > 0 {
      return 0, fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
   }
   if resp.Data == nil {
      return 0, fmt.Errorf("API returned null data")
   }

   return resp.Data.PopularTitles.TotalCount, nil
}

// executeGraphQL helper function
func executeGraphQL(payload map[string]interface{}) ([]byte, error) {
   jsonData, err := json.Marshal(payload)
   if err != nil {
      return nil, err
   }

   req, err := http.NewRequest("POST", "https://apis.justwatch.com/graphql", bytes.NewBuffer(jsonData))
   if err != nil {
      return nil, err
   }

   req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
   req.Header.Set("Accept", "application/graphql-response+json,application/json;q=0.9")
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("Origin", "https://www.justwatch.com")

   client := &http.Client{}
   resp, err := client.Do(req)
   if err != nil {
      return nil, err
   }
   defer resp.Body.Close()

   bodyBytes, err := io.ReadAll(resp.Body)
   if err != nil {
      return nil, err
   }

   if resp.StatusCode != http.StatusOK {
      return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
   }
   return bodyBytes, nil
}
