package main

import (
   "cmp"
   "encoding/json"
   "errors"
   "flag"
   "fmt"
   "io"
   "log"
   "net/http"
   "net/url"
   "os"
   "slices"
   "strings"
)

func get_slugs(data string) ([]string, error) {
   var slugs []string
   for {
      _, after, found := strings.Cut(data, ".slug=")
      if !found {
         break
      }
      slugVal, _, found := strings.Cut(after[2:], `\"`)
      if !found {
         return nil, errors.New("closing quote not found")
      }
      slugs = append(slugs, slugVal)
      // Advance the search window to find the next one
      data = after 
   }
   return slugs, nil
}

// processCountry fetches and parses provider data for a given country.
// It returns an ordered slice of provider slugs that match the filter, or an error.
func processCountry(countryCode string, providerFilter map[string]bool) ([]string, error) {
   resp, err := http.Get(fmt.Sprintf("https://www.justwatch.com/%s", countryCode))
   if err != nil {
      return nil, fmt.Errorf("failed to get URL for country %s: %w", countryCode, err)
   }
   defer resp.Body.Close()
   if resp.StatusCode != 200 {
      return nil, fmt.Errorf("request failed for country %s with status code: %d %s", countryCode, resp.StatusCode, resp.Status)
   }
   var data strings.Builder
   _, err = io.Copy(&data, resp.Body)
   if err != nil {
      return nil, err
   }
   slugs, err := get_slugs(data.String())
   if err != nil {
      return nil, err
   }
   var foundProviders []string
   for _, slug := range slugs {
      // If filter is nil (for -a flag) or the slug is in the filter, add it.
      if providerFilter == nil || providerFilter[slug] {
         foundProviders = append(foundProviders, slug)
      }
   }
   return foundProviders, nil
}

func main() {
   log.SetFlags(log.Ltime)
   http.DefaultTransport = &http.Transport{
      DisableKeepAlives: true, // github.com/golang/go/issues/25793
      Proxy: func(req *http.Request) (*url.URL, error) {
         log.Println(req.Method, req.URL)
         return nil, nil
      },
   }

   countryCode := flag.String("a", "", "Country code to process (e.g., 'us')")
   jsonFile := flag.String("b", "", "JSON file with a list of provider URLs")
   flag.Parse()

   if *countryCode == "" && *jsonFile == "" {
      flag.Usage()
      return
   }

   // Handle -a flag
   if *countryCode != "" {
      providers, err := processCountry(*countryCode, nil)
      if err != nil {
         log.Fatalf("failed to process country %s: %v", *countryCode, err)
      }
      for i, slug := range providers {
         fmt.Printf("%d. (%s) %s\n", i+1, *countryCode, slug)
      }
   }

   // Handle -b flag
   if *jsonFile != "" {
      processJSONFile(*jsonFile)
   }
}

// processJSONFile handles the logic for the -b flag: reading the file,
// parsing URLs, sorting countries, and fetching/filtering provider slugs.
func processJSONFile(filename string) {
   file, err := os.ReadFile(filename)
   if err != nil {
      log.Fatalf("failed to read json file: %v", err)
   }

   var providerURLs []string
   if err := json.Unmarshal(file, &providerURLs); err != nil {
      log.Fatalf("failed to unmarshal json file: %v", err)
   }

   countriesToProvidersList := make(map[string][]string)
   for _, providerURL := range providerURLs {
      parsedURL, err := url.Parse(providerURL)
      if err != nil {
         log.Printf("failed to parse URL %s: %v", providerURL, err)
         continue
      }
      pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
      if len(pathParts) != 3 {
         log.Printf("invalid provider URL format: %s", providerURL)
         continue
      }
      country, providerSlug := pathParts[0], pathParts[2]
      countriesToProvidersList[country] = append(countriesToProvidersList[country], providerSlug)
   }

   type CountryInfo struct {
      Code      string
      Providers []string
   }
   var sortedCountries []*CountryInfo
   for code, providers := range countriesToProvidersList {
      sortedCountries = append(sortedCountries, &CountryInfo{Code: code, Providers: providers})
   }
   slices.SortFunc(sortedCountries, func(a, b *CountryInfo) int {
      return cmp.Or(
         // Primary: Descending sort by length (b - a)
         len(b.Providers)-len(a.Providers),
         // Secondary: Ascending sort by Code
         cmp.Compare(a.Code, b.Code),
      )
   })

   type FinalResult struct {
      Country string
      Slug    string
   }
   var finalResults []FinalResult

   for _, countryInfo := range sortedCountries {
      if len(countryInfo.Providers) == 1 {
         finalResults = append(finalResults, FinalResult{Country: countryInfo.Code, Slug: countryInfo.Providers[0]})
         continue
      }

      providerFilter := make(map[string]bool)
      for _, slug := range countryInfo.Providers {
         providerFilter[slug] = true
      }

      foundSlugs, err := processCountry(countryInfo.Code, providerFilter)
      if err != nil {
         log.Printf("error processing country %s: %v", countryInfo.Code, err)
         continue
      }

      for _, slug := range foundSlugs {
         finalResults = append(finalResults, FinalResult{Country: countryInfo.Code, Slug: slug})
      }
   }

   // Print the final, consolidated list
   for i, result := range finalResults {
      fmt.Printf("%d. (%s) %s\n", i+1, result.Country, result.Slug)
   }
}
