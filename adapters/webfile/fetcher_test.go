package webfile

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestFetch(t *testing.T) {
	f := Fetcher{
		url: "https://raw.githubusercontent.com/flashbots/dowg/main/builder-registrations.json",
		cl:  http.Client{},
	}
	bts, err := f.Fetch(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(string(bts))
}
