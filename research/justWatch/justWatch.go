package main

import (
   "bytes"
   "encoding/json"
   "fmt"
   "io"
   "net/http"
)

func main() {
   url := "https://apis.justwatch.com/graphql"

   // Construct the GraphQL payload matching the target request
   payload := map[string]interface{}{
      "operationName": "GetPopularTitles",
      "variables": map[string]interface{}{
         "first":               40,
         "popularTitlesSortBy": "POPULAR",
         "sortRandomSeed":      0,
         "offset":              nil,
         "after":               "",
         "popularTitlesFilter": map[string]interface{}{
            "ageCertifications":          []string{},
            "excludeGenres":              []string{},
            "excludeProductionCountries": []string{},
            "objectTypes":                []string{},
            "productionCountries":        []string{},
            "subgenres":                  []string{},
            "genres":                     []string{},
            "packages":                   []string{"rtb"}, // Filtering for the specific package
            "excludeIrrelevantTitles":    false,
            "presentationTypes":          []string{},
            "monetizationTypes":          []string{},
            "searchQuery":                "",
         },
         "watchNowFilter": map[string]interface{}{
            "packages":          []string{"rtb"},
            "monetizationTypes": []string{},
         },
         "language": "fr",
         "country":  "BE",
      },
      // Using the exact query from the HAR to prevent WAF / Query-Hash rejections
      "query": `query GetPopularTitles($country: Country!, $first: Int! = 70, $format: ImageFormat, $language: Language!, $after: String, $popularTitlesFilter: TitleFilter, $popularTitlesSortBy: PopularTitlesSorting! = POPULAR, $profile: PosterProfile, $sortRandomSeed: Int! = 0, $watchNowFilter: WatchNowOfferFilter!, $offset: Int = 0) {
  popularTitles(
    country: $country
    filter: $popularTitlesFilter
    first: $first
    sortBy: $popularTitlesSortBy
    sortRandomSeed: $sortRandomSeed
    offset: $offset
    after: $after
  ) {
    __typename
    edges {
      cursor
      node {
        ...PopularTitleGraphql
        __typename
      }
      __typename
    }
    pageInfo {
      startCursor
      endCursor
      hasPreviousPage
      hasNextPage
      __typename
    }
    totalCount
  }
}


fragment PopularTitleGraphql on MovieOrShow {
  __typename
  id
  objectId
  objectType
  content(country: $country, language: $language) {
    title
    fullPath
    originalReleaseYear
    shortDescription
    interactions {
      likelistAdditions
      dislikelistAdditions
      __typename
    }
    scoring {
      imdbVotes
      imdbScore
      tmdbPopularity
      tmdbScore
      tomatoMeter
      certifiedFresh
      jwRating
      __typename
    }
    interactions {
      votesNumber
      __typename
    }
    posterUrl(profile: $profile, format: $format)
    isReleased
    runtime
    __typename
  }
  likelistEntry {
    createdAt
    __typename
  }
  dislikelistEntry {
    createdAt
    __typename
  }
  watchlistEntryV2 {
    createdAt
    __typename
  }
  customlistEntries {
    createdAt
    __typename
  }
  freeOffersCount: offerCount(
    country: $country
    platform: WEB
    filter: {monetizationTypes: [FREE, ADS]}
  )
  watchNowOffer(country: $country, platform: WEB, filter: $watchNowFilter) {
    ...WatchNowOffer
    __typename
  }
  ... on Movie {
    seenlistEntry {
      createdAt
      __typename
    }
    __typename
  }
  ... on Show {
    tvShowTrackingEntry {
      createdAt
      __typename
    }
    seenState(country: $country) {
      seenEpisodeCount
      progress
      __typename
    }
    __typename
  }
}


fragment WatchNowOffer on Offer {
  __typename
  id
  standardWebURL
  preAffiliatedStandardWebURL
  streamUrl
  streamUrlExternalPlayer
  package {
    id
    icon
    packageId
    clearName
    shortName
    technicalName
    iconWide(profile: S160)
    hasRectangularIcon(country: $country, platform: WEB)
    __typename
  }
  retailPrice(language: $language)
  retailPriceValue
  lastChangeRetailPriceValue
  currency
  presentationType
  monetizationType
  availableTo
  dateCreated
  newElementCount
}`,
   }

   jsonData, err := json.Marshal(payload)
   if err != nil {
      fmt.Println("Error marshaling payload:", err)
      return
   }

   req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
   if err != nil {
      fmt.Println("Error creating request:", err)
      return
   }

   // Important headers from HAR to mimic the browser
   req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
   req.Header.Set("Accept", "*/*")
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("Origin", "https://www.justwatch.com")
   req.Header.Set("Referer", "https://www.justwatch.com/")
   req.Header.Set("app-version", "3.13.0-web-web")
   req.Header.Set("device-id", "w6XCsMKCTAnDn0HDCsK8wq")
   req.Header.Set("sg", "c=BE&l=fr&pv=1c03d9a0-42c4-4443-8e72-390040102a6f&d=w6XCsMKCTAnDn0HDCsK8wq&p=3.13.0-web-web&pa=%2Fbe&e=")

   client := &http.Client{}
   resp, err := client.Do(req)
   if err != nil {
      fmt.Println("Error making request:", err)
      return
   }
   defer resp.Body.Close()

   body, err := io.ReadAll(resp.Body)
   if err != nil {
      fmt.Println("Error reading response:", err)
      return
   }

   // Struct tailored to specifically extract the totalCount field
   var responseData struct {
      Data struct {
         PopularTitles struct {
            TotalCount int `json:"totalCount"`
         } `json:"popularTitles"`
      } `json:"data"`
   }

   if err := json.Unmarshal(body, &responseData); err != nil {
      fmt.Println("Error parsing JSON:", err)
      return
   }

   fmt.Printf("Total Count: %d\n", responseData.Data.PopularTitles.TotalCount)
}
