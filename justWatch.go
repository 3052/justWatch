package main

import (
   "bytes"
   "cmp"
   _ "embed"
   "encoding/json"
   "errors"
   "maps"
   "net/http"
   "net/url"
   "slices"
   "strings"
)

//go:embed GetUrlTitleDetails.gql
var get_url_title_details string

var params_to_delete = []struct {
   date  string
   key   string
   value string
}{
   {"2026-03-08", "searchReferral", ""},
   {"2026-03-07", "referrer", "JustWatch"},
   {"2026-03-04", "subId3", "justappsvod"},
   {"2026-02-26", "autoplay", "1"},
   {"2026-02-26", "searchReferral", "publisher"},
   {"2026-02-26", "source", "bing"},
   {"2026-02-26", "source", "search-feeds"},
   {"2026-02-26", "utm_campaign", "vod_feed"},
   {"2026-02-26", "utm_content", ""},
   {"2026-02-26", "utm_medium", "deeplink"},
   {"2026-02-26", "utm_medium", "partner"},
   {"2026-02-26", "utm_source", "justWatch-v2-catalog"},
   {"2026-02-26", "utm_source", "justwatch"},
   {"2026-02-26", "utm_source", "universal_search"},
   {"2026-02-26", "utm_term", ""},
}

// https://justwatch.com/us/movie/goodfellas
func GetPath(rawUrl string) (string, error) {
   url_parse, err := url.Parse(rawUrl)
   if err != nil {
      return "", err
   }
   if url_parse.Scheme == "" {
      return "", errors.New("invalid URL: scheme is missing")
   }
   return url_parse.Path, nil
}

func GroupAndSortByUrl(offers []*EnrichedOffer) ([]string, map[string][]*EnrichedOffer) {
   groupedOffers := make(map[string][]*EnrichedOffer)
   for _, offer := range offers {
      key := getUrlGroupingKey(offer.Offer.StandardWebUrl)
      groupedOffers[key] = append(groupedOffers[key], offer)
   }
   for _, offerGroup := range groupedOffers {
      slices.SortFunc(offerGroup, func(a, b *EnrichedOffer) int {
         return cmp.Compare(a.Locale.Country, b.Locale.Country)
      })
   }
   // This works for Go 1.21 and older.
   keys := slices.SortedFunc(maps.Keys(groupedOffers), func(a, b string) int {
      return cmp.Compare(len(a), len(b))
   })
   return keys, groupedOffers
}

func getUrlGroupingKey(rawUrl string) string {
   trimmedUrl := strings.TrimSuffix(rawUrl, "\n")
   parsed, err := url.Parse(trimmedUrl)
   if err != nil {
      return trimmedUrl
   }
   if parsed.RawQuery == "" {
      return parsed.String()
   }
   query := parsed.Query()
   for _, rule := range params_to_delete {
      // .Get() returns the first value. If the key doesn't exist, it returns "".
      // This perfectly handles the "assume one value" rule.
      if query.Get(rule.key) == rule.value {
         delete(query, rule.key)
      }
   }
   parsed.RawQuery = query.Encode()
   return parsed.String()
}

type Content struct {
   HrefLangTags []HrefLangTag `json:"href_lang_tags"`
}

func (c *Content) Fetch(path string) error {
   req := http.Request{
      URL: &url.URL{
         Scheme:   "https",
         Host:     "apis.justwatch.com",
         Path:     "/content/urls",
         RawQuery: url.Values{"path": {path}}.Encode(),
      },
      Header: http.Header{},
   }
   resp, err := http.DefaultClient.Do(&req)
   if err != nil {
      return err
   }
   defer resp.Body.Close()
   if resp.StatusCode != http.StatusOK {
      return errors.New(resp.Status)
   }
   return json.NewDecoder(resp.Body).Decode(c)
}

type EnrichedOffer struct {
   Locale *Locale
   Offer  *Offer
}

// Deduplicate removes true duplicates where both the Offer and Locale are identical.
func Deduplicate(offers []*EnrichedOffer) []*EnrichedOffer {
   // 1. Sort the slice. This brings identical EnrichedOffers next to each other.
   // This part is correct as it compares the underlying values.
   slices.SortFunc(offers, func(a, b *EnrichedOffer) int {
      return cmp.Or(
         cmp.Compare(a.Offer.StandardWebUrl, b.Offer.StandardWebUrl),
         cmp.Compare(a.Offer.MonetizationType, b.Offer.MonetizationType),
         a.Offer.ElementCount-b.Offer.ElementCount,
         cmp.Compare(a.Locale.FullLocale, b.Locale.FullLocale),
      )
   })
   // 2. Compact the sorted slice, removing consecutive duplicates.
   return slices.CompactFunc(offers, func(a, b *EnrichedOffer) bool {
      return a.Offer.StandardWebUrl == b.Offer.StandardWebUrl &&
         a.Offer.MonetizationType == b.Offer.MonetizationType &&
         a.Offer.ElementCount == b.Offer.ElementCount &&
         a.Locale.FullLocale == b.Locale.FullLocale
   })
}

// FilterOffers removes offers with unwanted monetization types.
// If no unwantedTypes are provided, all offers are returned unfiltered.
func FilterOffers(offers []*EnrichedOffer, unwantedTypes ...string) []*EnrichedOffer {
   unwantedSet := make(map[string]struct{}, len(unwantedTypes))
   for _, unwanted := range unwantedTypes {
      if unwanted != "" {
         unwantedSet[unwanted] = struct{}{}
      }
   }
   var filteredOffers []*EnrichedOffer
   for _, offer := range offers {
      if _, found := unwantedSet[offer.Offer.MonetizationType]; !found {
         filteredOffers = append(filteredOffers, offer)
      }
   }
   return filteredOffers
}

type HrefLangTag struct {
   Href   string // /ar/pelicula/mulholland-drive
   Locale string // es_AR
}

func (h *HrefLangTag) Offers(localeVar *Locale) ([]Offer, error) {
   data, err := json.Marshal(map[string]any{
      "query": get_url_title_details,
      "variables": map[string]string{
         "country":  localeVar.Country,
         "fullPath": h.Href,
      },
   })
   if err != nil {
      return nil, err
   }
   resp, err := http.Post(
      "https://apis.justwatch.com/graphql", "application/json",
      bytes.NewReader(data),
   )
   if err != nil {
      return nil, err
   }
   if resp.StatusCode != http.StatusOK {
      var data strings.Builder
      err = resp.Write(&data)
      if err != nil {
         return nil, err
      }
      return nil, errors.New(data.String())
   }
   defer resp.Body.Close()
   var result struct {
      Data struct {
         Url struct {
            Node struct {
               Offers []Offer
            }
         }
      }
   }
   err = json.NewDecoder(resp.Body).Decode(&result)
   if err != nil {
      return nil, err
   }
   return result.Data.Url.Node.Offers, nil
}

type Offer struct {
   ElementCount     int
   MonetizationType string
   StandardWebUrl   string
}

// justwatch.go
