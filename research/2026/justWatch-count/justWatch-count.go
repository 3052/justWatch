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

//go:embed GetPopularTitles.graphql
var graphqlQuery string

// InputRequest models the expected structure of the input JSON file
type InputRequest struct {
   Package string `json:"package"`
   Country string `json:"country"`
}

// ResponseData models the structure to handle both successes and GraphQL errors
type ResponseData struct {
   Errors []struct {
      Message string `json:"message"`
   } `json:"errors"`
   // Data is a pointer so it can safely unmarshal a JSON null
   Data *struct {
      PopularTitles struct {
         TotalCount int `json:"totalCount"`
         Edges      []struct {
            Node struct {
               WatchNowOffer struct {
                  Package struct {
                     ClearName string `json:"clearName"`
                  } `json:"package"`
               } `json:"watchNowOffer"`
            } `json:"node"`
         } `json:"edges"`
      } `json:"popularTitles"`
   } `json:"data"`
}

func main() {
   fileFlag := flag.String("file", "", "Path to JSON file containing package/country array (required)")
   flag.Parse()

   // 1. Validate that the flag was actually provided
   if *fileFlag == "" {
      flag.Usage()
      log.Fatal("Error: the -file flag is required.")
   }

   // 2. Validate that the file actually exists before trying to read it
   if _, err := os.Stat(*fileFlag); os.IsNotExist(err) {
      log.Fatalf("Error: the file '%s' does not exist.", *fileFlag)
   }

   // 3. Safely read the file
   fileBytes, err := os.ReadFile(*fileFlag)
   if err != nil {
      log.Fatalf("Failed to read file '%s': %v", *fileFlag, err)
   }

   var requests []InputRequest
   if err := json.Unmarshal(fileBytes, &requests); err != nil {
      log.Fatalf("Failed to parse JSON data: %v", err)
   }

   for _, req := range requests {
      if req.Package == "" || req.Country == "" {
         log.Printf("Skipping invalid entry (missing package or country): %+v", req)
         continue
      }

      totalCount, providerName, err := fetchProviderData(req.Package, req.Country)
      if err != nil {
         log.Printf("[%s - %s] Failed: %v\n", req.Package, req.Country, err)
         continue
      }

      fmt.Printf("[%s - %s] Provider: %-15s | Total Count: %d\n", req.Package, req.Country, providerName, totalCount)
   }
}

func fetchProviderData(pkg, country string) (int, string, error) {
   variables := map[string]interface{}{
      "country":             country,
      "first":               1, // Only need 1 item to grab the provider name
      "popularTitlesSortBy": "POPULAR",
      "sortRandomSeed":      0,
      "offset":              0,
      "after":               "",
      "popularTitlesFilter": map[string]interface{}{
         "ageCertifications":          []string{},
         "excludeGenres":              []string{},
         "excludeProductionCountries": []string{},
         "objectTypes":                []string{},
         "productionCountries":        []string{},
         "subgenres":                  []string{},
         "genres":                     []string{},
         "packages":                   []string{pkg},
         "excludeIrrelevantTitles":    false,
         "presentationTypes":          []string{},
         "monetizationTypes":          []string{},
         "searchQuery":                "",
         "tomatoMeter": map[string]int{
            "min": 60,
         },
      },
      "watchNowFilter": map[string]interface{}{
         "packages":          []string{pkg},
         "monetizationTypes": []string{},
      },
   }

   payload := map[string]interface{}{
      "operationName": "GetPopularTitles",
      "variables":     variables,
      "query":         graphqlQuery,
   }

   jsonData, err := json.Marshal(payload)
   if err != nil {
      return 0, "", fmt.Errorf("error marshaling JSON payload: %w", err)
   }

   httpReq, err := http.NewRequest("POST", "https://apis.justwatch.com/graphql", bytes.NewBuffer(jsonData))
   if err != nil {
      return 0, "", fmt.Errorf("error creating request: %w", err)
   }

   httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
   httpReq.Header.Set("Accept", "application/graphql-response+json,application/json;q=0.9")
   httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
   httpReq.Header.Set("Content-Type", "application/json")
   httpReq.Header.Set("Origin", "https://www.justwatch.com")
   httpReq.Header.Set("Referer", "https://www.justwatch.com/")

   client := &http.Client{}
   resp, err := client.Do(httpReq)
   if err != nil {
      return 0, "", fmt.Errorf("HTTP request failed: %w", err)
   }
   defer resp.Body.Close()

   bodyBytes, err := io.ReadAll(resp.Body)
   if err != nil {
      return 0, "", fmt.Errorf("failed to read response body: %w", err)
   }

   var responseData ResponseData
   if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
      return 0, "", fmt.Errorf("error unmarshaling response JSON: %w\nBody: %s", err, string(bodyBytes))
   }

   // Handle GraphQL specific errors
   if len(responseData.Errors) > 0 {
      var errMsgs []string
      for _, e := range responseData.Errors {
         errMsgs = append(errMsgs, e.Message)
      }
      return 0, "", fmt.Errorf("GraphQL error: %s", strings.Join(errMsgs, " | "))
   }

   // Guard against nil data if there were no errors but data is somehow missing
   if responseData.Data == nil {
      return 0, "", fmt.Errorf("API returned null data without an explicit error message")
   }

   total := responseData.Data.PopularTitles.TotalCount
   provider := "Unknown"

   // Check if we got at least one edge back to extract the package name
   if len(responseData.Data.PopularTitles.Edges) > 0 {
      provider = responseData.Data.PopularTitles.Edges[0].Node.WatchNowOffer.Package.ClearName
   }

   return total, provider, nil
}
