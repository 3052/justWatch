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
)

//go:embed GetPopularTitles.graphql
var graphqlQuery string

// ResponseData models the stripped-down structure we need
type ResponseData struct {
   Data struct {
      PopularTitles struct {
         TotalCount int `json:"totalCount"`
      } `json:"popularTitles"`
   } `json:"data"`
}

func main() {
   // Define command line flags
   pkg := flag.String("package", "cpd", "JustWatch package code (e.g., cpd, nfx, hbm)")
   country := flag.String("country", "CZ", "Country code (e.g., CZ, US)")
   flag.Parse()

   // Call the extracted function
   totalCount, err := fetchTotalCount(*pkg, *country)
   if err != nil {
      log.Fatalf("Failed to get total count: %v", err)
   }

   fmt.Printf("Total Count: %d\n", totalCount)
}

// fetchTotalCount builds the GraphQL payload, makes the request, and parses the total count
func fetchTotalCount(pkg, country string) (int, error) {
   // 1. Construct the minimized GraphQL Variables
   // 2. Construct the full request payload using the embedded query
   jsonData, err := json.Marshal(map[string]any{
      "variables": map[string]any{
         "country": country,
         "popularTitlesFilter": map[string]any{
            "packages": []string{pkg},
         },
      },
      "query": graphqlQuery,
   })
   if err != nil {
      return 0, fmt.Errorf("error marshaling JSON payload: %w", err)
   }
   // 3. Create and configure the HTTP request
   req, err := http.NewRequest("POST", "https://apis.justwatch.com/graphql", bytes.NewBuffer(jsonData))
   if err != nil {
      return 0, fmt.Errorf("error creating request: %w", err)
   }
   // Adding standard headers mirroring the original request to avoid getting blocked
   req.Header.Set("Content-Type", "application/json")
   // 4. Execute the request
   client := &http.Client{}
   resp, err := client.Do(req)
   if err != nil {
      return 0, fmt.Errorf("HTTP request failed: %w", err)
   }
   defer resp.Body.Close()

   bodyBytes, err := io.ReadAll(resp.Body)
   if err != nil {
      return 0, fmt.Errorf("failed to read response body: %w", err)
   }
   log.Println("len(bodyBytes)", len(bodyBytes))
   if resp.StatusCode != http.StatusOK {
      return 0, fmt.Errorf("API returned non-200 status code: %d\nBody: %s", resp.StatusCode, string(bodyBytes))
   }
   // 5. Parse the response and return the totalCount
   var responseData ResponseData
   if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
      return 0, fmt.Errorf("error unmarshaling response JSON: %w", err)
   }

   return responseData.Data.PopularTitles.TotalCount, nil
}
