# snap-trip-interview

## 1. Introduction 
this is an interview project requested by [snap trip](https://www.snapptrip.com/) online traveling and hotel reservation agency for their 2022 sumer internship bootcamp,
you can read further about the project description details under project-document/[project_definition.pdf]([https://](https://github.com/nikanzamani/snap-trip-interview/blob/main/project-document/project_defenition.pdf))  
the gist of it is a ticket pricing engine that handles two types of json requests, 1) price change request that will take information about a list of tickets and return theme with a changed prices according to some predefined rules. 2) rule creation request that will take some parameters for price changing rules, previously mentioned.

## 2. Setup

to run the pricing engine you will need these prerequisites  
1) a postgreSQL database instance
2) a redis cache server instance

as for the main server, you can either compile it from the source code from [main.go](https://https://github.com/nikanzamani/snap-trip-interview/blob/main/main.go) file or use the precompiled executables from the [release](https://github.com/nikanzamani/snap-trip-interview/releases/tag/v1.0.0) page. alternatively you can also use the official docker image from [dockerhub](https://hub.docker.com/r/nikanz/snaptrip-interview) for easier deployment  

### 2.1. Local
after initialling both the postgres and redis servers you can either download the precompiled executable for linux and run it with commandline, mentioned operations  in Linux based operation systems would be as follows:  

```bash
wget https://github.com/nikanzamani/snap-trip-interview/releases/download/v1.0.0/snap-trip-interview
chmod +x snap-trip-interview 
./snap-trip-interview 
```
or you can build the project from source code as been shown bellow:
```bash
go mod download
go build 
./snap-trip-interview 
```
before running it though, you need to set some environment variables related to database and cache connection by either setting them directly or putting them in a `.env` file in the same directory as the main executable. you can see the full list of environment variables and their default values bellow. there is also an example `.env` file in the repository

 <table>
  <tr>
    <th>Environment Variable</th>
    <th>Default Value</th>
    <th>Description</th>
  </tr>
  <tr>
    <td>APP_PORT</td>
    <td>8080</td>
    <td>port on which the app is reachable</td>
  </tr>
  <tr>
    <td>REDIS_HOST</td>
    <td>localhost</td>
    <td>host in which redis is reachable</td>
  </tr>
  <tr>
    <td>REDIS_PORT</td>
    <td>6379</td>
    <td>port on which redis is reachable</td>
  </tr>
  <tr>
    <td>REDIS_PASSWORD</td>
    <td></td>
    <td>redis password needed for connection (it is unnecessary if the redis server password was not set)</td>
  </tr>
  <tr>
    <td>POSTGRES_HOST</td>
    <td>localhost</td>
    <td>host in which postgres is reachable</td>
  </tr>
  <tr>
    <td>POSTGRES_PORT</td>
    <td>5432</td>
    <td>port on which postgres is reachable</td>
  </tr>
  <tr>
    <td>POSTGRES_USER</td>
    <td>postgres</td>
    <td>postgres root user name</td>
  </tr>
  <tr>
    <td>POSTGRES_PASSWORD</td>
    <td></td>
    <td>postgres password needed for connection (it is necessary to be set)</td>
  </tr>
  <tr>
    <td>POSTGRES_DB_NAME</td>
    <td>POSTGRES_USER</td>
    <td>postgres db name for connection (the default value is the same as postgres user name)</td>
  </tr>
</table> 


### 2.2. Docker
you can see the full documentation in [dockerhub](https://hub.docker.com/r/nikanz/snaptrip-interview) 

## 3. Usage
to use the pricing engine you need to make an http post request to the related API endpoint. the endpoints are (`/price_request` and `/rule_creation`). there are example requests under project-document/[request-response](https://github.com/nikanzamani/snap-trip-interview/tree/main/project-document/request-response). relevant values for both request types (cities, agencies, airlines, suppliers) are also available at [valid](https://github.com/nikanzamani/snap-trip-interview/tree/main/valid)

## 4. Performance
to test the performance of the server you need to use some utility tool to make concurrent requests, I provide a python script to create random price change requests of random length and then post them concurrently and calculate request per second server performance. however when possible I suggest using industry standard tools, like apache bench. to use apache bench follow the steps blow

```bash
sudo apt install apache2-utils

# for price_request use the following
ab -n 1000 -c 20 -p price_request.json localhost:8080/price_request

# for rule_creation use the following
ab -n 1000 -c 20 -p rule_creation.json localhost:8080/rule_creation
```

also I suggest using local redis server for performance testing because of containerized redis servers high impact on performance