#!/bin/sh
# Полный логический бэкап БД picker → /backups (формат pg_dump -Fc, сжимается сам).
# Храним 14 дней, старые удаляем. Внешнее копирование/шифрование — за пределами стека
# (S3/рfbackrest/WAL-G), см. README.
set -eu

ts=$(date +%Y%m%d_%H%M%S)
name="picker_${ts}.dump"
target="/backups/${name}"

pg_dump -Fc -h postgres -U picker -d picker -f "$target"
echo "BACKUP_OK ${target} ($(du -h "$target" | cut -f1))"

# ротация: хранить 14 дней
find /backups -maxdepth 1 -name 'picker_*.dump' -mtime +14 -delete