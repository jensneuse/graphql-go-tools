package fauna

import (
	"fmt"
	f "github.com/fauna/faunadb-go/faunadb"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFauna(t *testing.T) {

	faunaSecret := os.Getenv("FAUNA_SECRET")
	if faunaSecret == "" {
		t.Fatal("must set env FAUNA_SECRET")
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(5) * time.Second,
		Transport: NewLoggingRoundTripper(),
	}

	client := f.NewFaunaClient(faunaSecret, f.HTTP(httpClient))

	result, err := client.Query(
		f.Obj{
			"data": f.Obj{
				"posts": f.Select(f.Arr{"data"}, f.Map(
					f.Paginate(
						f.Match(f.Index("posts")),
					),
					f.Lambda("post", f.Let(
						f.Obj{
							"post": f.Get(f.Var("post")),
						},
						f.Obj{
							"id":          f.Select(f.Arr{"ref", "id"}, f.Var("post")),
							"description": f.Select(f.Arr{"data", "description"}, f.Var("post")),
							"comments": f.Select("data", f.Map(
								f.Paginate(
									f.MatchTerm(f.Index("comment_by_post"), f.Select("ref", f.Var("post"))),
								),
								f.Lambda("comment", f.Let(
									f.Obj{
										"comment": f.Get(f.Var("comment")),
									}, f.Obj{
										"text": f.Select(f.Arr{"data", "text"}, f.Var("comment")),
									})),
							)),
						},
					)),
				)),
			},
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	out, err := prettyPrint(result)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("Response:\n\n%s", out)
}