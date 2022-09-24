package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

// TODO: respond with ID
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

const psqlConnect = "host=localhost port=5432 user=postgres password=password1234 dbname=postgres sslmode=disable"

// TODO:generalize psql connection

func main() {
	load_data()

	http.HandleFunc("/price_request", price_request)

	http.HandleFunc("/rule_creation", rule_creation)

	log.Fatal((http.ListenAndServe(":8080", nil)))
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
	defer rdb.Close()
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
	defer rdb.Close()
	err := rdb.Set(ctx, query+strconv.Itoa(valueType), amount, 0).Err()
	check_error(err)

	db, err := sql.Open("postgres", psqlConnect)
	check_error(err)
	defer db.Close()
	upsertStmt := `INSERT INTO snap (rule,amount) VALUES ( $1 ,$2)   
	ON CONFLICT (rule)
	DO UPDATE SET amount=$2;
	`
	_, e := db.Exec(upsertStmt, query+strconv.Itoa(valueType), amount)
	check_error(e)
}

func load_data() {

	fmt.Println("loading data from postgreSQL to redis...")

	db, err := sql.Open("postgres", psqlConnect)
	check_error(err)
	defer db.Close()
	creatTableStmt := `CREATE TABLE IF NOT EXISTS snap(
		id SERIAL PRIMARY KEY,
		rule VARCHAR(100) UNIQUE NOT NULL, 
		amount INT NOT NULL
	 );`
	_, e := db.Exec(creatTableStmt)
	check_error(e)

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	rows, err := db.Query(`SELECT "rule", "amount" FROM "snap"`)
	check_error(err)

	defer rows.Close()
	for rows.Next() {
		var rule string
		var amount int

		err = rows.Scan(&rule, &amount)
		check_error(err)

		err := rdb.Set(ctx, rule, amount, 0).Err()
		check_error(err)
	}
	check_error(err)

	fmt.Println("loading data is done!")
}

func check_error(err error) {
	if err != nil {
		panic(err)
	}
}
