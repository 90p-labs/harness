.PHONY: install test phpstan cs lint-container

install:
	docker compose run --rm -T php composer install --no-interaction --no-progress

test: install ## gate
	docker compose run --rm -T php vendor/bin/phpunit

phpstan: install ## gate
	docker compose run --rm -T php vendor/bin/phpstan analyse --no-progress --level=6 src

cs: install ## gate
	docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff

lint-container: install ## gate
	docker compose run --rm -T php php bin/console lint:container
