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
   inputFile := flag.String("input", "", "Path to the JSON file containing URLs (required)")
   flag.Parse()

   if *inputFile == "" {
      fmt.Println("Error: the -input flag is required.")
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

   // 3. Set the start and current year (last 10 years)
   currentYear := time.Now().Year()
   startYear := currentYear - 10

   // 4. Process URLs sequentially (one at a time)
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
      
      q := u.Query()
      q.Set("release_year_from", fmt.Sprintf("%d", startYear))
      u.RawQuery = q.Encode()
      targetURL := u.String()

      fmt.Printf("[%d/%d] Fetching %s...\n", i+1, len(urls), targetURL)

      // Pass the startYear down so ExtractTotalCount can find the correct Apollo key
      res, err := processURL(client, targetURL, startYear)
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

   // 5. Sort the results descending by TotalCount
   sort.Slice(results, func(i, j int) bool {
      return results[i].TotalCount > results[j].TotalCount
   })

   // 6. Output as a Markdown table
   fmt.Printf("\n## movies (%d - %d)\n\n", startYear, currentYear)
   fmt.Println("| Titles | Country | Provider |")
   fmt.Println("|---|---|---|")
   for _, r := range results {
      fmt.Printf("| %.0f | %s | %s |\n", r.TotalCount, r.Country, r.Provider)
   }
}

// ExtractTotalCount parses the Apollo state JSON byte slice and extracts
// the popular titles totalCount as a float64. It uses the requested year to
// ensure it gets the filtered count and not the default trending count.
func ExtractTotalCount(jsonData []byte, year int) (float64, error) {
   // Unmarshal into a generic map
   var data map[string]interface{}
   if err := json.Unmarshal(jsonData, &data); err != nil {
      return 0, fmt.Errorf("failed to parse JSON: %v", err)
   }

   // Drill down into the "defaultClient" object
   defaultClientVal, exists := data["defaultClient"]
   if !exists {
      return 0, fmt.Errorf("defaultClient key missing")
   }

   defaultClient, ok := defaultClientVal.(map[string]interface{})
   if !ok {
      return 0, fmt.Errorf("defaultClient is not a valid object")
   }

   expectedYearStr := fmt.Sprintf("%d", year)

   // Iterate through the keys inside defaultClient
   for key, value := range defaultClient {
      // Target the popularTitles query that specifically contains the releaseYear filter
      if strings.HasPrefix(key, "$ROOT_QUERY.popularTitles") && 
         strings.Contains(key, "releaseYear") && 
         strings.Contains(key, expectedYearStr) {
         
         // Type assert the value to a nested map
         if obj, isMap := value.(map[string]interface{}); isMap {
            
            // Check if "totalCount" exists inside this object
            if tc, hasTotalCount := obj["totalCount"]; hasTotalCount {
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

// Result holds the final data we want to output
type Result struct {
   Country    string
   Provider   string
   TotalCount float64
}

// processURL handles the fetching and parsing for a single URL
func processURL(client *http.Client, rawURL string, year int) (Result, error) {
   var result Result

   // Parse the URL to extract Country and Provider
   parsedURL, err := url.Parse(rawURL)
   if err != nil {
      return result, fmt.Errorf("invalid URL: %v", err)
   }

   // Path starts with "/", e.g., "/us/provider/disney-plus/movies"
   // Splitting yields: ["", "us", "provider", "disney-plus", "movies"]
   pathParts := strings.Split(parsedURL.Path, "/")
   if len(pathParts) >= 4 {
      result.Country = pathParts[1]
      // The provider name is always the 4th part of the path (index 3)
      result.Provider = pathParts[3]
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

   // Clean up potential trailing semicolon (e.g. `window.__APOLLO_STATE__={};`)
   // so the standard json.Unmarshal doesn't throw a syntax error.
   stateJSON = bytes.TrimSpace(stateJSON)
   stateJSON = bytes.TrimSuffix(stateJSON, []byte(";"))

   count, err := ExtractTotalCount(stateJSON, year)
   if err != nil {
      return result, err
   }

   result.TotalCount = count
   return result, nil
}

// --- Functions Provided By You Below ---

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
