package justWatch

import (
   "fmt"
   "testing"
)

func Test(t *testing.T) {
   locales_data, err := FetchLocales("en-US")
   if err != nil {
      t.Fatal(err)
   }
   for _, locale_data := range locales_data {
      fmt.Printf("%#v,\n", locale_data)
   }
   locale_data, ok := locales_data.Locale(&HrefLangTag{Locale: "en_US"})
   if !ok {
      t.Fatal("Locales.Locale")
   }
   fmt.Println(locale_data)
}
