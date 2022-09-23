package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
)

type priceChangeRequest struct {
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	Airline      string `json:"airline"`
	Agency       string `json:"agency"`
	Supplier     string `json:"supplier"`
	BasePrice    int    `json:"basePrice"`
	Markup       int    `json:"markup"`
	PayablePrice int    `json:"payablePrice"`
}
type route struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}
type ruleCreationRequest struct {
	Routes      []route  `json:"routes"`
	Airlines    []string `json:"airlines"`
	Agencies    []string `json:"agencies"`
	Suppliers   []string `json:"suppliers"`
	AmountType  string   `json:"amountType"`
	AmountValue int      `json:"amountValue"`
}

var ctx = context.Background()

func main() {

	http.HandleFunc("/price_request", price_request)

	http.HandleFunc("/rule_creation", rule_creation)

	http.ListenAndServe(":8080", nil)
}

func price_request(w http.ResponseWriter, r *http.Request) {
	var prices []priceChangeRequest
	json.NewDecoder(r.Body).Decode(&prices)
	prices = add_markups(prices)
	json.NewEncoder(w).Encode(prices)

}

func rule_creation(w http.ResponseWriter, r *http.Request) {
	var rules []ruleCreationRequest
	json.NewDecoder(r.Body).Decode(&rules)
	creat_rules(rules)
	// TODO:return correct response
	// json.NewEncoder(w).Encode(peter)
}

func add_markups(prices []priceChangeRequest) []priceChangeRequest {

	for ind, price := range prices {
		arrPrice := []string{price.Origin, price.Destination, price.Airline, price.Agency, price.Supplier}
		max_fixed := 0
		max_percent := 0
		for i := 0; i < 32; i++ {
			query := ""
			for j := 0; j < 5; j++ {
				if (i>>j)&1 == 1 {
					query = query + arrPrice[j]
				}
				query = query + ","
			}
			fixed, percent := get_rule(query)
			if fixed > max_fixed {
				max_fixed = fixed
			}
			if percent > max_percent {
				max_percent = percent
			}

		}
		if max_percent*price.BasePrice/100 > max_fixed {
			max_fixed = max_percent * price.BasePrice / 100
		}
		prices[ind].Markup = max_fixed
		prices[ind].PayablePrice = max_fixed + price.BasePrice
	}
	return prices
}
func creat_rules(rules []ruleCreationRequest) {
	// TODO: add error handling
	// city,airline,agency,supplier doesn't exist
	// amountType isn't correct

	for _, rule := range rules {
		if len(rule.Routes) == 0 {
			rule.Routes = append(rule.Routes, route{"", ""})
		}
		if len(rule.Airlines) == 0 {
			rule.Airlines = append(rule.Airlines, "")
		}
		if len(rule.Agencies) == 0 {
			rule.Agencies = append(rule.Agencies, "")
		}
		if len(rule.Suppliers) == 0 {
			rule.Suppliers = append(rule.Suppliers, "")
		}
		for _, route := range rule.Routes {
			for _, airline := range rule.Airlines {
				for _, agency := range rule.Agencies {
					for _, supplier := range rule.Suppliers {
						query := route.Origin + "," + route.Destination + "," + airline + "," + agency + "," + supplier + ","
						if rule.AmountType == "FIXED" {
							set_rule(query, 0, rule.AmountValue)
						} else if rule.AmountType == "PERCENTAGE" {
							set_rule(query, 1, rule.AmountValue)
						} else {
							// TODO:
						}
					}
				}
			}
		}
	}
}

func get_rule(query string) (fixed, percent int) {

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	var ans0, ans1 int
	val0, err := rdb.Get(ctx, query+"0").Result()
	if err == redis.Nil {
		ans0 = 0
	} else if err != nil {
		panic(err)
	} else {
		ans0, _ = strconv.Atoi(val0)
	}

	val1, err := rdb.Get(ctx, query+"1").Result()
	if err == redis.Nil {
		ans1 = 0
	} else if err != nil {
		panic(err)
	} else {
		ans1, _ = strconv.Atoi(val1)
	}

	return ans0, ans1
}
func set_rule(query string, valueType int, amount int) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	vals, err := rdb.Get(ctx, query+strconv.Itoa(valueType)).Result()
	if err == redis.Nil {
		vals = "0"
	} else if err != nil {
		panic(err)
	}
	vali, _ := strconv.Atoi(vals)
	if vali < amount {
		err := rdb.Set(ctx, query+strconv.Itoa(valueType), amount, 0).Err()
		if err != nil {
			panic(err)
		}
	}
}
