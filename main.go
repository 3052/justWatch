package main

import (
   "bytes"
   _ "embed"
   "encoding/base64"
   "encoding/json"
   "errors"
   "flag"
   "fmt"
   "log"
   "net/http"
   "os"
   "path"
   "strings"
   "time"
)

//go:embed BackendConstantsFetcherQuery.gql
var backend_constants_fetcher_query string

func main() {
   log.SetFlags(log.Ltime)
   err := new(client).do()
   if err != nil {
      log.Fatal(err)
   }
}

type Locale struct {
   FullLocale  string
   Country     string
   CountryName string
}

type Locales []Locale

func FetchLocales(language string) (Locales, error) {
   data, err := json.Marshal(map[string]any{
      "query": backend_constants_fetcher_query,
      "variables": map[string]string{
         "language": language,
      },
   })
   if err != nil {
      return nil, err
   }
   req, err := http.NewRequest(
      "POST", "https://apis.justwatch.com/graphql", bytes.NewReader(data),
   )
   if err != nil {
      return nil, err
   }
   req.Header.Set("content-type", "application/json")
   req.Header.Set(
      "device-id", base64.RawStdEncoding.EncodeToString(make([]byte, 16)),
   )
   resp, err := http.DefaultClient.Do(req)
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
         Locales Locales
      }
   }
   err = json.NewDecoder(resp.Body).Decode(&result)
   if err != nil {
      return nil, err
   }
   return result.Data.Locales, nil
}

func (l Locales) Locale(tag *HrefLangTag) (*Locale, bool) {
   for _, locale_data := range l {
      if locale_data.FullLocale == tag.Locale {
         return &locale_data, true
      }
   }
   return nil, false
}

type client struct {
   address string
   filters string
   sleep   time.Duration
}

func (c *client) do() error {
   flag.StringVar(&c.address, "a", "", "address")
   flag.DurationVar(&c.sleep, "s", 99*time.Millisecond, "sleep")
   flag.StringVar(&c.filters, "f", "BUY,CINEMA,FAST,RENT", "filters (use -f= for none)")
   flag.Parse()

   if c.address != "" {
      return c.do_address()
   }
   flag.Usage()
   return nil
}

func (c *client) do_address() error {
   url_path, err := GetPath(c.address)
   if err != nil {
      return err
   }
   var content Content
   err = content.Fetch(url_path)
   if err != nil {
      return err
   }
   var allEnrichedOffers []*EnrichedOffer
   for _, tag := range content.HrefLangTags {
      locale, ok := EnUs.Locale(&tag)
      if !ok {
         return errors.New("Locale")
      }
      log.Print(locale)
      offers, err := tag.Offers(locale)
      if err != nil {
         return err
      }
      for _, offer := range offers {
         allEnrichedOffers = append(allEnrichedOffers,
            &EnrichedOffer{Locale: locale, Offer: &offer},
         )
      }
      time.Sleep(c.sleep)
   }
   enrichedOffers := Deduplicate(allEnrichedOffers)
   // Empty filter string means no filtering — return all offers.
   var filters []string
   if c.filters != "" {
      filters = strings.Split(c.filters, ",")
   }
   enrichedOffers = FilterOffers(enrichedOffers, filters...)
   sortedUrls, groupedOffers := GroupAndSortByUrl(enrichedOffers)
   data := &bytes.Buffer{}
   for i, address := range sortedUrls {
      if i >= 1 {
         data.WriteString("\n\n")
      }
      data.WriteString("## ")
      data.WriteString(address)
      for _, enriched := range groupedOffers[address] {
         data.WriteByte('\n')
         data.WriteString("\ncountry: ")
         data.WriteString(enriched.Locale.Country)
         data.WriteString("\nname: ")
         data.WriteString(enriched.Locale.CountryName)
         data.WriteString("\nmonetization: ")
         data.WriteString(enriched.Offer.MonetizationType)
         if enriched.Offer.ElementCount >= 1 {
            data.WriteString("\ncount: ")
            fmt.Fprint(data, enriched.Offer.ElementCount)
         }
      }
   }
   name := path.Base(url_path) + ".md"
   log.Println("WriteFile", name)
   return os.WriteFile(name, data.Bytes(), os.ModePerm)
}

// main.go
