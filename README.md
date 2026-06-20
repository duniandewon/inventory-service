# Textile Factory MVP: Traceable Manufacturing & Logistics Engine

A backend application built with Go to track the lifecycle of raw materials as they move through a textile manufacturing supply chain.

Traditional inventory systems only track "what you buy" and "what you sell." This engine provides absolute visibility into the messy middle: Location, Transformation (e.g., Yards of fabric → Cut pieces → Finished garments), and Lineage Traceability.

## 🛠 Tech Stack

- **Language:** Go (Golang) 1.25+

- **Router:** `go-chi/chi`

- **Database:** PostgreSQL (Raw SQL via `database/sql` & `lib/pq`)

- **Cache/Sessions:** Redis

- **Infrastructure:** Docker & Docker Compose

- **Architecture:** Idiomatic Go Pragmatic Layering (Handler → Service → Repository)

## 📋 Prerequisites

To run this project locally, you only need:

- [Docker](https://docs.docker.com/get-docker/)

- [Docker Compose](https://docs.docker.com/compose/install/)

_(Go is only required if you plan to run the application outside of Docker)._

## 🚀 Getting Started (Local Development)

### 1. Clone the repository

```bash

git clone <your-repo-url>

cd textile-mvp

```

### 2. Set up environment variables

Copy the example environment file and fill in your secure credentials:

```bash

cp .env.example .env

```

Note: Make sure .env is listed in your .dockerignore and .gitignore files so credentials are not leaked.

### 3. Spin up the infrastructure

Use Docker Compose to build and start the Go API, PostgreSQL, Redis, and their respective GUI tools.

```bash

docker compose up --build -d

```

### 4. Verify Services are Running

Once the containers are up, you can access the following services:

- **Go API:** `http://localhost:8080/health`  

- **pgAdmin (Database GUI):** `http://localhost:5050`
  - Login with the `PGADMIN_EMAIL` and `PGADMIN_PASSWORD` defined in your `.env`.  

  - Add a new server with Host: `postgres`, Username: `postgres`, and Password from `DB_PASSWORD`.  

- **RedisInsight (Cache GUI):** `http://localhost:8001`
  - Add a Redis database with Host: `redis` and Port: `6379`.

## 🗄️ Database Management & Migrations

This project intentionally avoids heavy ORMs and automated migration CLI tools to maintain absolute control over the SQL execution.

**Schema changes are managed manually:**

1. All SQL schema updates are stored as sequentially numbered files in the `db/migrations/` folder (e.g., `001_initial_schema.sql`).
2. Before running new application code, manually execute these `.sql` scripts against the PostgreSQL database using pgAdmin or the `psql` CLI.

## 🏗️ Project Structure

The project follows the Standard Go Project Layout organized by business domain:

```text
textile-mvp/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: loads config, connects DB, starts HTTP server
├── internal/
│   ├── config/                  # Environment variable loading
│   ├── auth/                    # Domain: Users, OTP, Sessions
│   ├── inventory/               # Domain: Products (blueprints) and Physical Stock
│   ├── logistics/               # Domain: Work Orders & Delivery
│   └── partners/                # Domain: Vendors, Cutters, Tailors, Customers
├── db/
│   └── migrations/              # Raw SQL schema history
├── compose.yml                  # Docker infrastructure setup
├── Dockerfile                   # Multi-stage Go build
└── README.md
```

(Within each domain package, logic is separated into `handler.go`, `service.go`, and `repository.go`).
