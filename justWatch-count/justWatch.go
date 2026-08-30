package main

import (
   "bytes"
   _ "embed"
   "encoding/json"
   "fmt"
   "io"
   "net/http"
   "strings"
)

// TomatoMeterMin is the minimum Rotten Tomatoes score
const TomatoMeterMin = 50

//go:embed GetUrlMetadata.graphql
var metadataQuery string

// Global cache to prevent re-downloading the heavy provider list for the same locale
var providerCache = make(map[string][]Provider)

//go:embed GetProviderTop10TitlesFallback.graphql
var titlesQuery string

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

// fetchTotalCount runs the GetProviderTop10TitlesFallback query to get the catalog size
func fetchTotalCount(pkg, country string) (int, error) {
   variables := map[string]interface{}{
      "country":             country,
      "first":               0,
      "popularTitlesSortBy": "TRENDING",
      "popularTitlesFilter": map[string]interface{}{
         "packages": []string{pkg},
         "tomatoMeter": map[string]int{
            "min": TomatoMeterMin,
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

// resolvePackageFromPath fetches the provider list (using cache) and matches the slug
func resolvePackageFromPath(path, locale string) (string, string, error) {
   cleanPath := strings.TrimRight(path, "/")
   segments := strings.Split(cleanPath, "/")
   urlSlug := segments[len(segments)-1]

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

      providerCache[locale] = providers
   }

   for _, p := range providers {
      if p.Slug == urlSlug {
         return p.ShortName, p.ClearName, nil
      }
   }

   return "", "", fmt.Errorf("could not find provider with slug '%s' in locale '%s'", urlSlug, locale)
}

// Provider models the JustWatch REST API response
type Provider struct {
   ShortName     string `json:"short_name"`
   TechnicalName string `json:"technical_name"`
   ClearName     string `json:"clear_name"`
   Slug          string `json:"slug"`
}
