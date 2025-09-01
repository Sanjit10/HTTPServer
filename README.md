# HTTPServer

A HTTP server written in Go for the Chirpy application.

## Features

- User registration and authentication (JWT, refresh tokens)
- Chirp posting, retrieval, and deletion
- User upgrade via Polka webhooks
- Profanity filter for chirps
- Metrics endpoint
- API key validation for admin endpoints

## Project Structure

- `main.go`: Entry point and HTTP routing
- `internal/auth/`: Authentication logic (JWT, password hashing, API key)
- `internal/database/`: Database access (users, chirps, refresh tokens)
- `sql/`: SQL schema and queries
- `files/`: Static files served by the app

## Setup

1. **Install dependencies**  
   Make sure you have Go and PostgreSQL installed.

2. **Configure environment**  
   Copy `.env.example` to `.env` and set the required variables:
   - `DB_URL`
   - `PLATFORM`
   - `SECRET`
   - `POLKA_KEY`

3. **Run database migrations**  
   Apply the SQL files in `sql/schema/` to your database.

4. **Build and run**
   ```sh
   go build -o HTTPServer
   ./HTTPServer
   ```

## API Endpoints

- `POST /api/users` - Register a new user
- `POST /api/login` - Login and receive tokens
- `POST /api/refresh` - Refresh access token
- `POST /api/revoke` - Revoke refresh token
- `POST /api/chirps` - Create a chirp (JWT required)
- `GET /api/chirps` - List chirps (optional filters)
- `GET /api/chirps/{chirp_id}` - Get a single chirp
- `DELETE /api/chirps/{chirp_id}` - Delete a chirp (JWT required)
- `PUT /api/users` - Update user email/password (JWT required)
- `POST /api/polka/webhooks` - Handle Polka webhook (API key required)
- `GET /admin/metrics` - View metrics

## Testing

Run unit tests with:

```sh
go test ./internal/...
```

---

For more details, see the source files:

- [main.go](main.go)
- [internal/auth/](internal/auth/)
