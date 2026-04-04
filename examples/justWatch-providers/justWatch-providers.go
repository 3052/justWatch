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

// Result holds the final data we want to output
type Result struct {
   Country    string
   Provider   string
   TotalCount float64
}

func main() {
   // 1. Define and parse the command-line flag for the input file
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

   // 3. Process URLs sequentially (one at a time)
   var results []Result

   // Custom HTTP client with a timeout
   client := &http.Client{Timeout: 15 * time.Second}

   fmt.Println("Fetching data sequentially, please wait...")

   for i, targetURL := range urls {
      fmt.Printf("[%d/%d] Fetching %s...\n", i+1, len(urls), targetURL)
      
      res, err := processURL(client, targetURL)
      if err != nil {
         log.Printf("  -> Error: %v\n", err)
         continue
      }

      results = append(results, res)
   }

   // 4. Sort the results descending by TotalCount
   sort.Slice(results, func(i, j int) bool {
      return results[i].TotalCount > results[j].TotalCount
   })

   // 5. Output the formatted list
   fmt.Printf("\n%-10s | %-30s | %s\n", "COUNTRY", "PROVIDER", "TOTAL COUNT")
   fmt.Println(strings.Repeat("-", 60))
   for _, r := range results {
      fmt.Printf("%-10s | %-30s | %.0f\n", r.Country, r.Provider, r.TotalCount)
   }
}

// processURL handles the fetching and parsing for a single URL
func processURL(client *http.Client, rawURL string) (Result, error) {
   var result Result

   // Parse the URL to extract Country and Provider
   parsedURL, err := url.Parse(rawURL)
   if err != nil {
      return result, fmt.Errorf("invalid URL: %v", err)
   }

   // path looks like "/us/provider/disney-plus" or "/se/leverantör/draken-films"
   pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
   if len(pathParts) >= 3 {
      result.Country = pathParts[0]
      // The provider name is always the last part of the path
      result.Provider = pathParts[len(pathParts)-1]
   } else {
      return result, errors.New("unexpected URL structure")
   }

   // Create request and set User-Agent to avoid being blocked
   req, err := http.NewRequest("GET", rawURL, nil)
   if err != nil {
      return result, err
   }
   req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")

   // Execute HTTP request
   resp, err := client.Do(req)
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

   // Feed state into the provided ExtractTotalCount function
   count, err := ExtractTotalCount(stateJSON)
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

func ExtractTotalCount(jsonData []byte) (float64, error) {
   // Unmarshal into a generic map
   var data map[string]interface{}
   if err := json.Unmarshal(jsonData, &data); err != nil {
      return 0, fmt.Errorf("failed to parse JSON: %v", err)
   }
   // Iterate through the top-level keys
   for key, value := range data {
      // Specifically target the popularTitles query to avoid grabbing
      // the totalCount from streamingCharts
      if strings.HasPrefix(key, "$ROOT_QUERY.popularTitles") {
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
