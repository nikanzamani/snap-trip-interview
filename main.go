package main

import (
	"bufio"
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
	"strings"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

// TODO: add apache bench explanation to readme
type priceChangeRequest struct {
	Id           *int   `json:"id"`
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
	Status  string  `json:"status"`
	Message *string `json:"message"`
}

func (r ruleCreationRequest) validate_rule() (b bool) {
	for _, route := range r.Routes {
		if !(search_validation(Vcities, strings.ToUpper(route.Origin)) && search_validation(Vcities, strings.ToUpper(route.Destination))) {
			return false
		}
	}
	for _, agency := range r.Agencies {
		if !(search_validation(Vagencies, strings.ToUpper(agency))) {
			return false
		}
	}
	for _, airline := range r.Airlines {
		if !(search_validation(Vairlines, strings.ToUpper(airline))) {
			return false
		}
	}
	for _, supplier := range r.Suppliers {
		if !(search_validation(Vsuppliers, strings.ToUpper(supplier))) {
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

var psqlConnect string
var redis_opt redis.Options

func main() {
	load_env()

	load_data()

	read_validation()

	http.HandleFunc("/test", test)

	http.HandleFunc("/price_request", price_request)

	http.HandleFunc("/rule_creation", rule_creation)

	log.Fatal((http.ListenAndServe(":8080", nil)))
}

func test(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello there\n")
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
		max_id_fixed := 0
		max_id_percent := 0
		for i := 0; i < 32; i++ {
			query := ""
			for j := 0; j < 5; j++ {
				if (i>>j)&1 == 1 {
					query = query + strings.ToUpper(arrPrice[j])
				}
				query = query + ","
			}
			id, fixed, percent := get_rule(query)
			if fixed > max_fixed {
				max_fixed = fixed
				max_id_fixed = id
			}
			if percent > max_percent {
				max_percent = percent
				max_id_percent = id
			}

		}
		if max_percent*price.BasePrice/100 > max_fixed {
			max_fixed = max_percent * price.BasePrice / 100
			max_id_fixed = max_id_percent
		}
		prices[ind].Markup = max_fixed
		prices[ind].PayablePrice = max_fixed + price.BasePrice
		if max_fixed != 0 {
			prices[ind].Id = newInt(max_id_fixed)
		}
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
			return ruleCreationResponse{Status: "FAILED", Message: newString("invalid rule at index: " + strconv.Itoa(ind))}
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

func get_rule(query string) (id, fixed, percent int) {

	rdb := redis.NewClient(&redis_opt)
	defer rdb.Close()

	val, err := rdb.Get(ctx, query).Result()
	if err == redis.Nil {
		id = 0
		fixed = 0
		percent = 0
	} else if err != nil {
		panic(err)
	} else {
		ans := strings.Split(val, ",")
		id, _ = strconv.Atoi(ans[0])
		fixed, _ = strconv.Atoi(ans[1])
		percent, _ = strconv.Atoi(ans[2])
	}

	return
}

func set_rule(query string, valueType int, amount int) {
	db, err := sql.Open("postgres", psqlConnect)
	check_error(err)
	defer db.Close()

	rows, err := db.Query(`SELECT "id", "fixed", "percent" FROM "snap" WHERE rule=$1`, query)
	check_error(err)

	id := 0
	fixed := 0
	percent := 0
	for rows.Next() {
		err = rows.Scan(&id, &fixed, &percent)
		check_error(err)
	}

	if valueType == 0 {
		fixed = amount
	} else {
		percent = amount
	}

	upsertStmt := `INSERT INTO snap (rule,fixed,percent) VALUES ( $1, $2, $3)
	ON CONFLICT (rule)
	DO UPDATE SET fixed=$2, percent=$3;
	`
	_, e := db.Exec(upsertStmt, query, fixed, percent)
	check_error(e)

	rdb := redis.NewClient(&redis_opt)
	defer rdb.Close()
	err = rdb.Set(ctx, query, strconv.Itoa(id)+","+strconv.Itoa(fixed)+","+strconv.Itoa(percent), 0).Err()
	check_error(err)

}

func load_data() {

	fmt.Println("loading data from postgreSQL to redis...")

	db, err := sql.Open("postgres", psqlConnect)
	check_error(err)
	defer db.Close()
	creatTableStmt := `CREATE TABLE IF NOT EXISTS snap(
		id SERIAL PRIMARY KEY,
		rule VARCHAR(100) UNIQUE NOT NULL, 
		fixed INT NOT NULL,
		percent INT NOT NULL
		);`
	_, e := db.Exec(creatTableStmt)
	check_error(e)

	rdb := redis.NewClient(&redis_opt)

	rows, err := db.Query(`SELECT "id", "rule", "fixed", "percent" FROM "snap"`)
	check_error(err)

	defer rows.Close()
	for rows.Next() {
		var id int
		var rule string
		var fixed int
		var percent int

		err = rows.Scan(&id, &rule, &fixed, &percent)
		check_error(err)

		err := rdb.Set(ctx, rule, strconv.Itoa(id)+","+strconv.Itoa(fixed)+","+strconv.Itoa(percent), 0).Err()
		check_error(err)
	}
	fmt.Println("loading data is done!")
}

func load_env() {
	if _, err := os.Stat(".env"); err == nil {
		file, err := os.Open(".env")
		check_error(err)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			e := strings.Split(scanner.Text(), "=")
			if len(e) == 2 {
				os.Setenv(e[0], e[1])
			}
		}
		err = scanner.Err()
		check_error(err)
	}
	// redis env
	env_exist_default("REDIS_HOST", "localhost")
	env_exist_default("REDIS_PORT", "6379")
	//postgres env
	env_exist_default("POSTGRES_HOST", "localhost")
	env_exist_default("POSTGRES_PORT", "5432")
	env_exist_default("POSTGRES_USER", "postgres")
	env_exist_default("POSTGRES_DB_NAME", os.Getenv("POSTGRES_USER"))
	psqlConnect = "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB_NAME sslmode=disable"
	psqlConnect = os.ExpandEnv(psqlConnect)

	redis_opt = redis.Options{
		Addr:     os.ExpandEnv("$REDIS_HOST:$REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	}
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

func env_exist_default(key, default_val string) {
	if _, ok := os.LookupEnv(key); !ok {
		os.Setenv(key, default_val)
	}
}

func newString(s string) *string {
	return &s
}
func newInt(i int) *int {
	return &i
}
