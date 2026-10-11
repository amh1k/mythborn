.PHONY: api backend backend-stop

# Start only the HTTP API. Temporal and the frontend are managed separately.
api:
	docker compose -f docker-compose.backend.yml up --build api

# Start the API and worker against the Temporal endpoint configured in .env.
backend:
	docker compose -f docker-compose.backend.yml up --build

backend-stop:
	docker compose -f docker-compose.backend.yml down
