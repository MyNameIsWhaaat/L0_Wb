# L0_Wb
This repo is for the purpose of studying kafka streaming in Go.
# Task
Look file: task.txt
# Start instructions
1. Clone the repository locally to any directory on your device git clone https://github.com/MyNameIsWhaaat/L0_Wb.git
2. Change to the project directory manually or using the console cd wbL0
3. Build & run docker containers docker-compose build && docker-compose up OR using a Make utility make docker
4. After starting the containers, all entities will be automatically created in the database using a gorm auto migration
5. To start the main service, run the following command: go run ./cmd/subscriber
6. Once the container is launched, the Swagger html page will also be available for the convenience of API testing
http://localhost:8081/swagger/index.html#/
7. You can access the simple web UI for displaying and interacting with the orders at: http://localhost:8081/
8. To publish messages to Kafka, run the publisher: go run ./cmd/publisher
# Technologies
* Golang
* Kafka
* Gin
* Gorm
* PostgreSQL
* Swagger
* Docker
# HTTP methods:
* Get the order from the database
* Get the order from the cache
* Get all orders from the cache
# Request examples:
# Get the order from the database - method GET
```http://localhost:8081/api/order/db/:uid```
For example input path parameter - uid -> b563feb7b2b84b6test
Output
```
{
  "order_uid": "b563feb7b2b84b6teST",
  "track_number": "WBILMTESTTRACK",
  "entry": "WBIL",
  "locale": "en",
  "internal_signature": "",
  "customer_id": "test",
  "delivery_service": "meest",
  "shardkey": "9",
  "sm_id": 99,
  "date_created": "2021-11-26T06:22:19Z",
  "oof_shard": "1",
  "delivery": {
    "name": "Test Testov",
    "phone": "+9720000000",
    "zip": "2639809",
    "city": "Kiryat Mozkin",
    "address": "Ploshad Mira 15",
    "region": "Kraiot",
    "email": "test@gmail.com"
  },
  "payment": {
    "transaction": "b563feb7b2b84b6test",
    "request_id": "",
    "currency": "USD",
    "provider": "wbpay",
    "amount": 1817,
    "payment_dt": 1637907727,
    "bank": "alpha",
    "delivery_cost": 1500,
    "goods_total": 317,
    "custom_fee": 0
  },
  "items": [
    {
      "chrt_id": 9934930,
      "track_number": "WBILMTESTTRACK",
      "price": 453,
      "rid": "ab4219087a764ae0btest",
      "name": "Mascaras",
      "sale": 30,
      "size": "0",
      "total_price": 317,
      "nm_id": 2389212,
      "brand": "Vivienne Sabo",
      "status": 202
    }
  ]
}
```
# Get the order from the cache - method GET
```http://localhost:8081/api/order/:uid```
Input parameter uid
Output same as from the method Get the order from the database

# Get all orders from the cache - method GET
go to all methods
```http://localhost:8081/api/orders```

Output
```
{
  "order_uid": "b563feb7b2b84b6teST",
  "track_number": "WBILMTESTTRACK",
  "entry": "WBIL",
  "locale": "en",
  "internal_signature": "",
  "customer_id": "test",
  "delivery_service": "meest",
  "shardkey": "9",
  "sm_id": 99,
  "date_created": "2021-11-26T06:22:19Z",
  "oof_shard": "1",
  "delivery": {
    "name": "Test Testov",
    "phone": "+9720000000",
    "zip": "2639809",
    "city": "Kiryat Mozkin",
    "address": "Ploshad Mira 15",
    "region": "Kraiot",
    "email": "test@gmail.com"
  },
  "payment": {
    "transaction": "b563feb7b2b84b6test",
    "request_id": "",
    "currency": "USD",
    "provider": "wbpay",
    "amount": 1817,
    "payment_dt": 1637907727,
    "bank": "alpha",
    "delivery_cost": 1500,
    "goods_total": 317,
    "custom_fee": 0
  },
  "items": [
    {
      "chrt_id": 9934930,
      "track_number": "WBILMTESTTRACK",
      "price": 453,
      "rid": "ab4219087a764ae0btest",
      "name": "Mascaras",
      "sale": 30,
      "size": "0",
      "total_price": 317,
      "nm_id": 2389212,
      "brand": "Vivienne Sabo",
      "status": 202
    }
  ]
}
```
