from urllib import request
from random import randint,choice
from time import perf_counter
from concurrent.futures import ThreadPoolExecutor
import os
import csv
import json

print("I advise using \"apache bench\" as detailed in the readme, instead of this in-house solution")


if os.path.isfile(".env"):
    with open(".env") as f:
        envs=f.read().split("\n")        
        for env in envs:
            e=env.split("=")
            if len(e)==2:
                os.environ[e[0]]=e[1]

if os.environ["APP_PORT"]=="":
    os.environ["APP_PORT"]=="8080"

cities=[]
airlines=[]
agencies=[]
suppliers=[]

with open("valid/city.csv","r") as f:
    csvreader=csv.reader(f)
    for row in csvreader:
        cities.append(row[2])

with open("valid/airline.csv","r") as f:
    csvreader=csv.reader(f)
    for row in csvreader:
        airlines.append(row[0])

with open("valid/agency.csv","r") as f:
    csvreader=csv.reader(f)
    for row in csvreader:
        agencies.append(row[2])

with open("valid/supplier.csv","r") as f:
    csvreader=csv.reader(f)
    for row in csvreader:
        suppliers.append(row[2])
all_req=[]
total_req=0
for _ in range(100):
    n=randint(10,20)
    total_req+=n
    req=[]
    for __ in range(n):
        k={
            "origin":choice(cities) ,
            "destination": choice(cities),
            "airline": choice(airlines),
            "agency": choice(agencies),
            "supplier": choice(suppliers),
            "basePrice": int(randint(1,20)*1e6),
            "markup": 0,
            "payablePrice": 0
        }
        req.append(k)
    data=json.dumps(req).encode("utf8")
    req =  request.Request(f"http://localhost:{os.environ['APP_PORT']}/price_request", data=data)
    all_req.append(req)


t1=perf_counter()
with ThreadPoolExecutor() as executor:
    a=executor.map(request.urlopen,all_req)
t2=perf_counter()

print(f"request per second: {round(total_req/(t2-t1),2)}")
print(f"total requests: {total_req}")