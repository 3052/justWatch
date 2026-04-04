package main

import (
   "bytes"
   "encoding/json"
   "errors"
   "fmt"
   "io"
   "net/http"
   "net/url"
   "strings"
)

// Result holds the final data we want to output
type Result struct {
   Country    string
   Provider   string
   TotalCount float64
}

// processURL handles the fetching and parsing for a single URL
func processURL(client *http.Client, rawURL string) (Result, error) {
   var result Result

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
   // Drill down into the "defaultClient" object
   defaultClient, ok := data["defaultClient"].(map[string]interface{})
   if !ok {
      return 0, fmt.Errorf("defaultClient key missing or invalid format")
   }
   // Iterate through the keys inside defaultClient
   for key, value := range defaultClient {
      // Target the popularTitles query
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
