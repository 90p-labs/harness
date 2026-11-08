.PHONY: install cs phpstan lint-container test

install: ## setup creates: vendor/autoload.php
	docker compose run --rm -T php composer install --no-interaction --no-progress

cs: ## gate
	docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff

phpstan: ## gate
	docker compose run --rm -T php vendor/bin/phpstan analyse --no-progress --level=6 src

lint-container: ## gate
	docker compose run --rm -T php php bin/console lint:container

test: ## gate needs: cs phpstan lint-container
	docker compose run --rm -T php vendor/bin/phpunit
