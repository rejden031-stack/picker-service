#!/bin/sh
# Восстановление из бэкапа: ./restore.sh <файл.dump> [новое_имя_бд]
#   файл — имя внутри /backups (например picker_20260926_020000.dump)
#   новое имя — куда залить; по умолчанию picker_restored.
# Перед восстановлением БД пересоздаётся с нуля.
set -eu

file=${1:?usage: restore <file.dump> [dbname]}
db=${2:-picker_restored}

pg_dump --version

psql -h postgres -U picker -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS ${db};" \
  -c "CREATE DATABASE ${db};"

pg_restore -h postgres -U picker -d "$db" --no-owner --no-acl "/backups/${file}"

echo "RESTORE_OK db=${db} file=${file}"
echo "Проверка (кол-во записей):"
psql -h postgres -U picker -d "$db" -tAc \
  "SELECT 'floors='||count(*) FROM floors UNION ALL SELECT 'racks='||count(*) FROM racks UNION ALL SELECT 'products='||count(*) FROM products;"