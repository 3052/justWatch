package main

import (
   "bytes"
   "encoding/json"
   "flag"
   "fmt"
   "io"
   "log"
   "net/http"
)

// ResponseData models the stripped-down structure we need
type ResponseData struct {
   Data struct {
      PopularTitles struct {
         TotalCount int `json:"totalCount"`
      } `json:"popularTitles"`
   } `json:"data"`
}

func main() {
   // 1. Define command line flags
   pkg := flag.String("package", "cpd", "JustWatch package code (e.g., cpd, nfx, hbm)")
   country := flag.String("country", "CZ", "Country code (e.g., CZ, US)")
   flag.Parse()

   // 2. Construct the minimized GraphQL Variables
   variables := map[string]interface{}{
      "country":             *country,
      "first":               1, // Minimized since we don't need the actual items
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
         "packages":                   []string{*pkg},
         "excludeIrrelevantTitles":    false,
         "presentationTypes":          []string{},
         "monetizationTypes":          []string{},
         "searchQuery":                "",
      },
   }

   // 3. Construct the full request payload
   payload := map[string]interface{}{
      "operationName": "GetPopularTitles",
      "variables":     variables,
      "query":         graphqlQuery,
   }

   jsonData, err := json.Marshal(payload)
   if err != nil {
      log.Fatalf("Error marshaling JSON payload: %v", err)
   }

   // 4. Create and configure the HTTP request
   req, err := http.NewRequest("POST", "https://apis.justwatch.com/graphql", bytes.NewBuffer(jsonData))
   if err != nil {
      log.Fatalf("Error creating request: %v", err)
   }

   // Adding standard headers mirroring the original request to avoid getting blocked
   req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
   req.Header.Set("Accept", "application/graphql-response+json,application/json;q=0.9")
   req.Header.Set("Accept-Language", "en-US,en;q=0.9")
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("Origin", "https://www.justwatch.com")
   req.Header.Set("Referer", "https://www.justwatch.com/")

   // 5. Execute the request
   client := &http.Client{}
   resp, err := client.Do(req)
   if err != nil {
      log.Fatalf("HTTP request failed: %v", err)
   }
   defer resp.Body.Close()

   bodyBytes, err := io.ReadAll(resp.Body)
   if err != nil {
      log.Fatalf("Failed to read response body: %v", err)
   }

   if resp.StatusCode != http.StatusOK {
      log.Fatalf("API returned non-200 status code: %d\nBody: %s", resp.StatusCode, string(bodyBytes))
   }

   // 6. Parse the response and print the totalCount
   var responseData ResponseData
   if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
      log.Fatalf("Error unmarshaling response JSON: %v", err)
   }

   fmt.Printf("Total Count: %d\n", responseData.Data.PopularTitles.TotalCount)
}

// The minimized GraphQL query with the signature cleanly reformatted
const graphqlQuery = `query GetPopularTitles(
  $country: Country!
  $popularTitlesFilter: TitleFilter
  $popularTitlesSortBy: PopularTitlesSorting! = POPULAR
  $first: Int! = 40
  $sortRandomSeed: Int! = 0
  $offset: Int = 0
  $after: String
) {
  popularTitles(
    country: $country
    filter: $popularTitlesFilter
    first: $first
    sortBy: $popularTitlesSortBy
    sortRandomSeed: $sortRandomSeed
    offset: $offset
    after: $after
  ) {
    totalCount
  }
}`
