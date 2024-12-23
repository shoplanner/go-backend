package repo

import (
	"context"
	"testing"

	"github.com/kr/pretty"
	"github.com/redis/go-redis/v9"
)

func Test(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Protocol: 2,
	})
	// redis.E
	resp, err := client.FTSearch(context.Background(), "idx:auth.RefreshToken", "@id:{a5c28b12\\-cf25\\-4697\\-8d3a\\-3a80eb00e199}").Result()
	if err != nil {
		t.Fatal(err)
	}

	pretty.Println(resp.Docs[0].Fields["$"])
}
