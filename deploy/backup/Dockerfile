# Контейнер ежедневных бэкапов Postgres: pg_dump -Fc + ротация по возрасту.
# Запуск вручную:  docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm backup
FROM postgres:16-alpine

# не наследуем docker-entrypoint.sh от постгреса — crond должен быть чистым PID 1
ENTRYPOINT []

RUN apk add --no-cache bash
COPY backup.sh /usr/local/bin/backup
COPY restore.sh /usr/local/bin/restore
COPY crontab /etc/crontabs/root

CMD ["crond", "-f", "-l", "8"]