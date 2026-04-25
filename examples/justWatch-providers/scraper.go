package main

import (
   "bytes"
   "encoding/json"
   "errors"
   "flag"
   "fmt"
   "io"
   "log"
   "net/http"
   "net/url"
   "os"
   "sort"
   "strings"
   "time"
)

func main() {
   // 1. Define and parse the command-line flags
   inputFile := flag.String("i", "", "Path to the JSON file containing URLs (required)")
   flag.Parse()

   if *inputFile == "" {
      flag.Usage()
      os.Exit(1)
   }

   // 2. Read the JSON file containing the URLs
   fileData, err := os.ReadFile(*inputFile)
   if err != nil {
      log.Fatalf("Failed to read file '%s': %v", *inputFile, err)
   }

   var urls []string
   if err := json.Unmarshal(fileData, &urls); err != nil {
      log.Fatalf("Failed to parse JSON in '%s': %v", *inputFile, err)
   }

   // 3. Process URLs sequentially (one at a time)
   var results []Result

   // Custom HTTP client with a timeout
   client := &http.Client{Timeout: 15 * time.Second}

   fmt.Println("Fetching data sequentially, please wait...")

   for i, rawTargetURL := range urls {
      // Safely parse the URL and add the query string parameter
      u, err := url.Parse(rawTargetURL)
      if err != nil {
         log.Printf("[%d/%d] Skipping invalid URL %s: %v\n", i+1, len(urls), rawTargetURL, err)
         continue
      }

      query := u.Query()
      query.Set("tomatoMeter", "80")
      u.RawQuery = query.Encode()
      targetURL := u.String()

      fmt.Printf("[%d/%d] %s...\n", i+1, len(urls), targetURL)

      res, err := processURL(client, targetURL)
      if err != nil {
         // If it's the specific totalCount error, halt the entire program immediately
         if strings.Contains(err.Error(), "totalCount not found in JSON") {
            log.Fatalf("  -> Fatal Error: %v. Stopping immediately.", err)
         }

         // For other errors (like timeouts), just log and continue
         log.Printf("  -> Error: %v\n", err)
         continue
      }

      results = append(results, res)
   }

   // 4. Sort the results descending by TotalCount
   sort.Slice(results, func(i, j int) bool {
      return results[i].TotalCount > results[j].TotalCount
   })

   // 5. Output as a Markdown table with Reference Links
   fmt.Println("\n| Titles | Country | Provider |")
   fmt.Println("|---|---|---|")
   for _, r := range results {
      // Format the Provider column as a Markdown reference link: [provider]
      fmt.Printf("| %.0f | %s | [%s] |\n", r.TotalCount, r.Country, r.Provider)
   }

   fmt.Println() // Empty line between the table and the references

   // Deduplicate and output markdown reference URLs
   printedRefs := make(map[string]bool)
   for _, r := range results {
      if !printedRefs[r.Provider] {
         fmt.Printf("[%s]:%s\n", r.Provider, r.URL)
         printedRefs[r.Provider] = true
      }
   }
}

// Result holds the final data we want to output
type Result struct {
   Country    string
   Provider   string
   TotalCount float64
   URL        string
}

// processURL handles the fetching and parsing for a single URL
func processURL(client *http.Client, rawURL string) (Result, error) {
   var result Result

   // Save the URL to our Result struct
   result.URL = rawURL

   // Parse the URL to extract Country and Provider
   parsedURL, err := url.Parse(rawURL)
   if err != nil {
      return result, fmt.Errorf("invalid URL: %v", err)
   }

   // Path starts with "/", e.g., "/us/provider/disney-plus"
   // Splitting yields: ["", "us", "provider", "disney-plus"]
   pathParts := strings.Split(parsedURL.Path, "/")
   if len(pathParts) >= 3 {
      result.Country = pathParts[1]
      // The provider name is always the last part of the path
      result.Provider = pathParts[len(pathParts)-1]
   } else {
      return result, errors.New("unexpected URL structure")
   }

   // Execute HTTP GET request
   resp, err := client.Get(rawURL)
   if err != nil {
      return result, err
   }
   defer resp.Body.Close()

   if resp.StatusCode != http.StatusOK {
      return result, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
   }

   htmlBody, err := io.ReadAll(resp.Body)
   if err != nil {
      return result, err
   }

   // Feed body into the provided ExtractApolloState function
   stateJSON, err := ExtractApolloState(htmlBody)
   if err != nil {
      return result, err
   }

   // Feed state into ExtractTotalCount
   count, err := ExtractTotalCount(stateJSON)
   if err != nil {
      return result, err
   }

   result.TotalCount = count
   return result, nil
}

func ExtractApolloState(html []byte) ([]byte, error) {
   _, after, found := bytes.Cut(html, []byte("window.__APOLLO_STATE__="))
   if !found {
      return nil, errors.New("__APOLLO_STATE__ not found in HTML")
   }
   state, _, found := bytes.Cut(after, []byte("</script>"))
   if !found {
      return nil, errors.New("closing script tag not found")
   }
   return state, nil
}

// ExtractTotalCount looks for the popularTitles query containing the tomatoMeter filter
func ExtractTotalCount(jsonData []byte) (float64, error) {
   // Unmarshal into a generic map
   var data map[string]interface{}
   if err := json.Unmarshal(jsonData, &data); err != nil {
      return 0, fmt.Errorf("failed to parse JSON: %v", err)
   }

   // Drill down into the "defaultClient" object
   defaultClient, ok := data["defaultClient"].(map[string]interface{})
   if !ok {
      return 0, fmt.Errorf("defaultClient key missing or invalid format")
   }

   // Iterate through the keys inside defaultClient
   for key, value := range defaultClient {
      // Target the popularTitles query that specifically contains the tomatoMeter filter we applied in the URL
      if strings.HasPrefix(key, "$ROOT_QUERY.popularTitles") && strings.Contains(key, "tomatoMeter") {

         // Type assert the value to a nested map
         if obj, isMap := value.(map[string]interface{}); isMap {
            // Check if "totalCount" exists inside this object
            if tc, exists := obj["totalCount"]; exists {
               // encoding/json parses JSON numbers as float64 by default
               if totalCount, isFloat := tc.(float64); isFloat {
                  return totalCount, nil
               }
            }
         }
      }
   }
   return 0, fmt.Errorf("totalCount not found in JSON")
}
