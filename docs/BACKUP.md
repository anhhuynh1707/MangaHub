# MangaHub SQLite backup and recovery

Status: **online backup and atomic restore passed in an isolated local Compose
project; first EC2 rehearsal pending**.

MangaHub uses SQLite in WAL mode. Copying only `mangahub.db` while the services
are running can omit committed pages still present in the WAL, so the backup
path uses SQLite's online backup API instead of `cp` or `tar`.

## Storage model

| Data | Production location | Purpose |
|---|---|---|
| Live SQLite | `mangahub-prod_mangahub-data` | Application source of truth |
| Verified backups | `mangahub-prod_mangahub-backups` | Separate logical recovery points |
| Release records | `/var/lib/mangahub-deploy` | Root-only operation audit |

The two named volumes are separate but initially live on the same encrypted EC2
root EBS volume. This protects against bad releases and accidental logical data
changes, not instance or EBS loss. Export to an encrypted off-instance location
is still required before termination; S3 is a future phase and is not claimed as
implemented.

Backups contain password hashes and user data. Files and checksum sidecars are
owned by UID `10001`, use mode `0600`, and must never be committed or uploaded
to a public portfolio repository.

## Create and list backups

From the pinned source checkout on EC2:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/backup.sh
sudo ./deploy/scripts/backup.sh --list
```

The default retention is the seven newest verified backup/checksum pairs. Set a
different positive count for one run:

```bash
sudo ./deploy/scripts/backup.sh 14
```

Each filename contains UTC time and the full deployed Git SHA. A backup is
published only after all of these pass:

1. SQLite online backup completes within the timeout.
2. The result is converted to a standalone database file.
3. `PRAGMA integrity_check` returns exactly `ok`.
4. The file is synced and atomically renamed.
5. A SHA-256 sidecar is synced beside it.

`/var/lib/mangahub-deploy/backups.tsv` records the UTC timestamp, release SHA,
and backup filename without recording application data.

## Restore one verified backup

Copy the exact filename from `backup.sh --list`:

```bash
sudo ./deploy/scripts/restore.sh \
  mangahub-YYYYMMDDTHHMMSSZ-sha-FULL_40_CHARACTER_SHA.db
```

Restore deliberately causes brief downtime. The script:

1. acquires both backup and deployment locks before reading release state;
2. validates filename, SHA-256 sidecar, and SQLite integrity before downtime;
3. creates an automatic pre-restore recovery point;
4. stops the edge, API, TCP, UDP, and gRPC services;
5. removes only stale SQLite WAL/SHM sidecars;
6. atomically replaces the main database;
7. restarts the same recorded application SHA and base/raw mode;
8. verifies every service plus the public health gate;
9. records the target and recovery-point names in `restores.tsv`.

If an error occurs after replacement, the exit trap attempts to restore the
pre-restore database and restart the recorded release. Preserve both volumes
and investigate if that automatic recovery reports a critical failure.

## Local rehearsal evidence

The isolated project `mangahub-backup-test` was exercised on 2026-08-21 UTC
(2026-08-22 in the project timezone):

- a live WAL database with 200 manga and 0 users was backed up;
- the 303,104-byte backup passed integrity and SHA-256 verification;
- a disposable account changed the source to 1 user after the backup;
- restore created a second verified pre-restore backup, stopped all DB clients,
  atomically restored the first backup, and passed the application health gate;
- the restored database returned to 0 users while retaining 200 manga;
- every backup and sidecar was mode `0600`, owned by `10001:10001`;
- a missing but syntactically valid backup was rejected before any service was
  stopped, and the application remained healthy;
- the isolated containers, networks, and volumes were removed after proof.

This evidence completes the local backup/restore gate only. Repeat the same
controlled account-count proof on EC2 before presenting AWS recovery as tested.

## Rules

- Never use `docker compose down -v` for normal deploy, rollback, backup, or
  restore.
- Never restore while any database-using service is still running.
- Never treat a database file without its matching checksum sidecar as verified.
- Never terminate the instance until a wanted backup has been exported and
  independently checked.
- Image rollback and data restore solve different failures; do not substitute
  one for the other.
