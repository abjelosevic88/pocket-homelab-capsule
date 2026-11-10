# Backup

Everything lives in `DATA_DIR`: the SQLite file `pocket-capsule.db` (WAL mode; every picture, recording and file is inside it — a capsule has to outlive the apps things came from). Take a consistent copy with `pocket-capsule backup` (writes `DATA_DIR/backups/pocket-capsule-<time>.db` with `VACUUM INTO`) or Settings → Backup & data → Download database (admin). Back up that copy, not the live file. `DATA_DIR/core.token*` is the Pocket Core registration.

Restore: stop the app, replace `pocket-capsule.db` (remove the `-wal`/`-shm` files), start.

Every person can also export the capsules they made, whole (everything inside with its files, who each is for, the history, the yearly capsules), as JSON (`/api/v1/export.json`, back in with Settings → Backup & data — twice is the same as once; sealed capsules stay sealed). What is inside a sealed capsule is packed in the file (gzip + base64) so a glance does not spoil it — it is not encrypted, so keep exports private. A CSV of the capsules is at `/api/v1/export.csv`.
