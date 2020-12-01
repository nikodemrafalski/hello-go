package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type randomOrgResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		Random struct {
			Data []int `json:"data"`
		} `json:"random"`
	} `json:"result"`
}

type resultItem struct {
	Data   []int   `json:"data"`
	StdDev float64 `json:"stddev"`
}

func getRandomsFromRandomOrg(ctx context.Context, count int) ([]int, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY env variable missing")
	}
	url := "https://api.random.org/json-rpc/2/invoke"
	payload := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"method\":\"generateIntegers\",\"params\":{\"apiKey\":\"%s\",\"n\":%d,\"min\":1,\"max\":100,\"replacement\":true,\"base\":10},\"id\":32404}", apiKey, count)

	request, requestError := http.NewRequest("POST", url, strings.NewReader(payload))
	if requestError != nil {
		return nil, requestError
	}
	request.Header.Add("content-type", "application/json")
	response, responseError := http.DefaultClient.Do(request.WithContext(ctx))
	if responseError != nil {
		return nil, responseError
	}

	defer response.Body.Close()
	var result randomOrgResponse
	unmarshallErr := json.NewDecoder(response.Body).Decode(&result)
	if unmarshallErr != nil {
		return nil, unmarshallErr
	}
	if result.Error.Code != 0 {
		return nil, fmt.Errorf("Random.org error code: %d message: %s", result.Error.Code, result.Error.Message)
	}

	return result.Result.Random.Data, nil
}

func getRandomSets(ctx context.Context, setsCount int, setSize int) ([][]int, error) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	errorsChannel := make(chan error, setsCount)
	defer cancel()
	sets := make([][]int, setsCount)
	for i := 0; i < setsCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result, err := getRandomsFromRandomOrg(ctx, setSize)
			if err != nil {
				errorsChannel <- err
				cancel()
				return
			}
			sets[id] = result
		}(i)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, <-errorsChannel
	}
	return sets, nil
}

func stdDev(set []int) float64 {
	count := len(set)
	var sum int
	for _, value := range set {
		sum += value
	}
	mean := float64(sum) / float64(count)
	var stdDev float64

	for _, value := range set {
		stdDev += math.Pow(float64(value)-mean, 2)
	}

	return math.Sqrt(stdDev / float64(count))
}

func calculateResults(sets [][]int) []resultItem {
	resultLength := len(sets) + 1
	results := make([]resultItem, resultLength)

	var union = []int{}
	for i, set := range sets {
		results[i] = resultItem{set, stdDev(set)}
		union = append(union, set...)
	}

	results[resultLength-1] = resultItem{union, stdDev(union)}
	return results
}

func main() {
	r := gin.Default()
	r.GET("/random/mean", func(c *gin.Context) {
		queryL := c.Query("length")
		queryR := c.Query("requests")
		if queryL == "" || queryR == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		length, err := strconv.Atoi(queryL)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		requests, err := strconv.Atoi(queryR)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		sets, err := getRandomSets(c, requests, length)
		if err != nil {
			log.Println(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		result := calculateResults(sets)
		c.JSON(200, result)
	})
	r.Run()
}
