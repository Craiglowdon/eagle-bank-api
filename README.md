# Eagle Bank API

A REST API for a fictional bank, implemented in Go using the standard `net/http` package and SQLite.

The API allows users to register and authenticate, manage their own bank accounts, and record deposits and withdrawals as immutable transactions.

## Current status

Implemented:

* Health endpoint
* User registration
* Email/password login
* JWT creation and validation
* Fetch authenticated user details
* User ownership and not-found handling
* Create bank accounts
* List accounts belonging to the authenticated user
* Fetch an individual owned account
* Account ownership and not-found handling
* Create deposit and withdrawal transactions
* Atomic account balance and transaction updates
* Insufficient-funds handling
* List transactions belonging to an account
* Fetch an individual transaction
* Transaction-to-account validation
* Transaction ownership and not-found handling
* SQLite persistence and database constraints
* Request validation and structured error responses
* Automated HTTP, database and authentication middleware tests

In progress:

* Remaining transaction creation and listing validation scenarios
* Update and delete users
* Update and delete bank accounts
* Final OpenAPI updates and contract verification

## Implemented endpoints

| Method | Path                                                        | Authentication |
| ------ | ----------------------------------------------------------- | -------------- |
| `GET`  | `/health`                                                   | No             |
| `POST` | `/v1/users`                                                 | No             |
| `POST` | `/v1/auth/login`                                            | No             |
| `GET`  | `/v1/users/{userId}`                                        | Bearer JWT     |
| `POST` | `/v1/accounts`                                              | Bearer JWT     |
| `GET`  | `/v1/accounts`                                              | Bearer JWT     |
| `GET`  | `/v1/accounts/{accountNumber}`                              | Bearer JWT     |
| `POST` | `/v1/accounts/{accountNumber}/transactions`                 | Bearer JWT     |
| `GET`  | `/v1/accounts/{accountNumber}/transactions`                 | Bearer JWT     |
| `GET`  | `/v1/accounts/{accountNumber}/transactions/{transactionId}` | Bearer JWT     |


## Requirements

* Go 1.26 or later
* No external database installation is required

## Running locally

Set a JWT signing secret containing at least 32 characters.

Git Bash or other Unix-like shells:

```bash
export JWT_SECRET="replace-with-a-secure-secret-at-least-32-characters"
go run .
```

PowerShell:

```powershell
$env:JWT_SECRET = "replace-with-a-secure-secret-at-least-32-characters"
go run .
```

The API listens on:

```text
http://localhost:8080
```

A persistent SQLite database named `eagle-bank.db` is created in the current working directory.

Check that the API is running:

```bash
curl -i http://localhost:8080/health
```

## Running the tests

```bash
go test ./...
```

Disable Go's test-result cache for a fresh run:

```bash
go test -count=1 ./...
```

Run tests with individual test names displayed:

```bash
go test -v -count=1 ./...
```

Tests use isolated temporary SQLite databases which are removed automatically after each test.

## Authentication

Users authenticate using their email address and password:

```http
POST /v1/auth/login
Content-Type: application/json
```

```json
{
  "email": "test@example.com",
  "password": "secure-password"
}
```

A successful login returns a signed JWT:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

Protected endpoints require the token as a bearer token:

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

JWTs:

* Use the HS256 signing algorithm
* Are signed using the `JWT_SECRET` environment variable
* Identify the authenticated user through the `sub` claim
* Use `eagle-bank-api` as the issuer
* Expire one hour after issue

Passwords are hashed using bcrypt and are never stored or returned in plaintext.

## Design decisions

### Standard-library HTTP routing

The API uses Go's `net/http` package and its method-aware routing rather than introducing an additional HTTP framework.

Handlers are wrapped with authentication middleware where bearer authentication is required.

### SQLite persistence

SQLite was selected to make the application straightforward to run without requiring a separate database service.

The pure-Go `modernc.org/sqlite` driver avoids requiring CGO or a C compiler.

The development database is file-backed and persists between application restarts. Automated tests use separate temporary databases.

### Monetary values

Monetary balances are stored as integer pennies:

```text
£10.99 → 1099
```

This avoids using binary floating-point values for persisted financial calculations.

Values are converted to the numeric representation required by the OpenAPI contract only at the API boundary.

### Transaction consistency

Creating a transaction updates two pieces of persisted state:

* The immutable transaction record
* The account balance

Both operations run inside one SQL transaction. The transaction is committed only after both operations succeed; otherwise, all changes are rolled back.

Withdrawals with insufficient funds return `422 Unprocessable Entity` without creating a transaction or changing the account balance. Deposits that would exceed the contract’s maximum account balance are rejected in the same way.


### Ownership and authorisation

The authenticated user ID comes from the verified JWT `sub` claim. Clients do not supply ownership identifiers when creating accounts or transactions.

Handlers use that identity to ensure users can access only their own resources.

Authentication and authorisation failures are distinguished:

* `401 Unauthorized` for missing, malformed, invalid or expired credentials
* `403 Forbidden` when an authenticated user attempts to access another user's resource

### Database constraints

Business validation is performed by the application so clients receive useful API errors.

Database constraints provide an additional integrity boundary. This includes:

* Unique user email addresses
* Valid account-number format
* Fixed sort code and currency values
* Non-negative account balances
* Foreign-key ownership relationships
* Prevention of deleting a user while bank accounts still reference them

### Dependency management

The implementation favours the Go standard library and uses external dependencies only where a dedicated implementation is required:

* `modernc.org/sqlite` for the SQLite database driver
* `golang.org/x/crypto` for bcrypt password hashing
* `github.com/golang-jwt/jwt/v5` for JWT creation and validation

## OpenAPI assumptions and discrepancies

The supplied OpenAPI specification and scenario document leave authentication credentials undefined while requiring an endpoint that authenticates a user and returns a JWT.

To provide a coherent authentication flow:

* A required, write-only `password` property is added to `CreateUserRequest`
* `POST /v1/auth/login` is added
* Login request and response schemas are added
* Passwords and password hashes are excluded from all user responses

In a production delivery, this change to the API contract would be agreed with the API owner before implementation.

The scenario document refers to account paths using `{accountId}`, while the supplied OpenAPI specification uses `{accountNumber}`. The implementation follows the OpenAPI path:

```text
/v1/accounts/{accountNumber}
```

Other apparent schema inconsistencies will be documented and corrected in the submitted OpenAPI specification where necessary.

## Project structure

```text
.
├── database/     SQLite setup, embedded schema and database tests
├── handlers/     HTTP endpoint handlers
├── middleware/   JWT bearer authentication
├── models/       API request and response models
├── server/       Route construction and API-level tests
├── main.go       Configuration and process startup
├── openapi.yaml  API contract
└── README.md
```

`main.go` is intentionally limited to loading configuration, opening the database and starting the HTTP server. Route assembly lives in the `server` package, while endpoint behaviour lives in handlers.

## Error responses

General errors use:

```json
{
  "message": "user not found"
}
```

Validation errors include field-level details:

```json
{
  "message": "invalid request",
  "details": [
    {
      "field": "email",
      "message": "email is required",
      "type": "required"
    }
  ]
}
```

## API specification

The API contract is defined in `openapi.yaml`.

The final implementation will be checked against the updated specification, including paths, methods, authentication requirements, request and response schemas, validation constraints and documented status codes.
