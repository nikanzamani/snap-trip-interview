package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

// TODO: respond with ID
// TODO: add apache bench explanation to readme
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
type ruleCreationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (r ruleCreationRequest) validate_rule() (b bool) {
	for _, route := range r.Routes {
		if !(search_validation(Vcities, route.Origin) && search_validation(Vcities, route.Destination)) {
			return false
		}
	}
	for _, agency := range r.Agencies {
		if !(search_validation(Vagencies, agency)) {
			return false
		}
	}
	for _, airline := range r.Airlines {
		if !(search_validation(Vairlines, airline)) {
			return false
		}
	}
	for _, supplier := range r.Suppliers {
		if !(search_validation(Vsuppliers, supplier)) {
			return false
		}
	}
	if r.AmountType != "PERCENTAGE" && r.AmountType != "FIXED" {
		return false
	}
	return true
}

var Vcities []string
var Vairlines []string
var Vagencies []string
var Vsuppliers []string

var ctx = context.Background()

const psqlConnect = "host=localhost port=5432 user=postgres password=password1234 dbname=postgres sslmode=disable"

// TODO: generalize psql connection
// TODO: combine percent and fixed in one call

func main() {
	load_data()

	read_validation()

	http.HandleFunc("/test", test)

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
	resp := creat_rules(rules)
	json.NewEncoder(w).Encode(resp)
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
func creat_rules(rules []ruleCreationRequest) (r ruleCreationResponse) {
	// TODO: ask ...
	// should replace rule with lower amount?
	// should return error message with detail?
	// is normal for docker redis to slow down so much?

	for ind, rule := range rules {
		b := rule.validate_rule()
		if !b {
			return ruleCreationResponse{Status: "FAILED", Message: "invalid rule at index: " + strconv.Itoa(ind)}
		}
	}

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
						}
					}
				}
			}
		}
	}
	return ruleCreationResponse{Status: "SUCCESS"}
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
	fmt.Println("loading data is done!")
}

func read_validation() {
	file, err := os.Open("valid/city.csv")
	check_error(err)
	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()
	for _, row := range records {
		Vcities = append(Vcities, row[2])
	}
	Vcities = append(Vcities, "")
	sort.Slice(Vcities, func(i, j int) bool { return Vcities[i] < Vcities[j] })

	file, err = os.Open("valid/agency.csv")
	check_error(err)
	reader = csv.NewReader(file)
	records, _ = reader.ReadAll()
	for _, row := range records {
		Vagencies = append(Vagencies, row[2])
	}
	Vagencies = append(Vagencies, "")
	sort.Slice(Vagencies, func(i, j int) bool { return Vagencies[i] < Vagencies[j] })

	file, err = os.Open("valid/airline.csv")
	check_error(err)
	reader = csv.NewReader(file)
	records, _ = reader.ReadAll()
	for _, row := range records {
		Vairlines = append(Vairlines, row[0])
	}
	Vairlines = append(Vairlines, "")
	sort.Slice(Vairlines, func(i, j int) bool { return Vairlines[i] < Vairlines[j] })

	file, err = os.Open("valid/supplier.csv")
	check_error(err)
	reader = csv.NewReader(file)
	records, _ = reader.ReadAll()
	for _, row := range records {
		Vsuppliers = append(Vsuppliers, strings.ToUpper(row[2]))
	}
	Vsuppliers = append(Vsuppliers, "")
	sort.Slice(Vsuppliers, func(i, j int) bool { return Vsuppliers[i] < Vsuppliers[j] })
}

func search_validation(v []string, q string) (b bool) {
	i := sort.Search(len(v), func(j int) bool { return v[j] >= q })
	return v[i] == q
}

func check_error(err error) {
	if err != nil {
		panic(err)
	}
}
func test(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "hello there\n")
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
	})
	defer rdb.Close()
	a, _ := rdb.Ping(ctx).Result()

	fmt.Fprint(w, a)

	db, err := sql.Open("postgres", psqlConnect)
	check_error(err)
	defer db.Close()
	b := db.Ping()

	fmt.Fprint(w, b)

}
