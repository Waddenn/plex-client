# build_cache.py
import os
import sqlite3
import argparse
import time
from plexapi.server import PlexServer

CONFIG_DIR = os.path.expanduser("~/.config/plex-minimal")
CACHE_DIR = os.path.expanduser("~/.cache/plex-minimal")
os.makedirs(CONFIG_DIR, exist_ok=True)
os.makedirs(CACHE_DIR, exist_ok=True)
TOKEN_PATH = os.path.join(CONFIG_DIR, "token")
BASEURL_PATH = os.path.join(CONFIG_DIR, "baseurl")
DB_PATH = os.path.join(CACHE_DIR, "cache.db")


def save_config(path, value):
    with open(path, "w") as f:
        f.write(value.strip())


def load_config(path, missing_msg=None):
    try:
        with open(path) as f:
            return f.read().strip()
    except FileNotFoundError:
        if missing_msg:
            print(missing_msg)
        return None


def parse_args():
    parser = argparse.ArgumentParser(description="Generate minimal Plex database")
    parser.add_argument("--baseurl", help="Plex server URL")
    parser.add_argument("--token", help="Plex authentication token")
    parser.add_argument("--debug", action="store_true", help="Show detailed logs")
    parser.add_argument("--force", action="store_true", help="Force cache rebuild even if cache is fresh")  # ajout
    return parser.parse_args()


args = parse_args()


def log_debug(msg):
    if args.debug:
        print(f"[DEBUG] {msg}")


if args.baseurl:
    save_config(BASEURL_PATH, args.baseurl)
if args.token:
    save_config(TOKEN_PATH, args.token)

baseurl = load_config(BASEURL_PATH, "❌ Missing baseurl.")
token = load_config(TOKEN_PATH, "❌ Missing token.")
if not baseurl or not token:
    exit(1)

log_debug("Connecting to Plex server...")
plex = PlexServer(baseurl, token)
log_debug("Connection established.")

def cache_is_fresh(db_path, max_age_hours=24):
    if not os.path.exists(db_path):
        return False
    mtime = os.path.getmtime(db_path)
    age = time.time() - mtime
    return age < max_age_hours * 3600


if cache_is_fresh(DB_PATH) and not args.force:
    print("✅ Cache is fresh (<24h), skipping rebuild.")
    exit(0)

with sqlite3.connect(DB_PATH) as conn:
    cur = conn.cursor()
    cur.executescript(
        """
        CREATE TABLE IF NOT EXISTS films (
            id INTEGER PRIMARY KEY,
            title TEXT,
            year INTEGER,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL,
            genres TEXT,
            originallyAvailableAt TEXT
        );

        CREATE TABLE IF NOT EXISTS series (
            id INTEGER PRIMARY KEY,
            title TEXT,
            summary TEXT,
            rating REAL,
            genres TEXT
        );

        CREATE TABLE IF NOT EXISTS saisons (
            id INTEGER PRIMARY KEY,
            serie_id INTEGER,
            saison_index INTEGER,
            summary TEXT
        );

        CREATE TABLE IF NOT EXISTS episodes (
            id INTEGER PRIMARY KEY,
            saison_id INTEGER,
            episode_index INTEGER,
            title TEXT,
            part_key TEXT,
            duration INTEGER,
            summary TEXT,
            rating REAL
        );
        """
    )

    # Récupérer les IDs existants pour éviter de re-télécharger les métadonnées
    cur.execute("SELECT id FROM films")
    existing_film_ids = set(row[0] for row in cur.fetchall())
    cur.execute("SELECT id FROM series")
    existing_series_ids = set(row[0] for row in cur.fetchall())
    cur.execute("SELECT id FROM saisons")
    existing_saison_ids = set(row[0] for row in cur.fetchall())
    cur.execute("SELECT id FROM episodes")
    existing_episode_ids = set(row[0] for row in cur.fetchall())

    film_insert = []
    series_insert = []
    saison_insert = []
    episode_insert = []

    for section in plex.library.sections():
        if section.type == "movie":
            for movie in section.all():
                if int(movie.ratingKey) not in existing_film_ids:
                    try:
                        p = movie.media[0].parts[0]
                        genres = ", ".join([g.tag for g in movie.genres]) if movie.genres else ""
                        film_insert.append((
                            movie.ratingKey,
                            movie.title,
                            movie.year,
                            p.key,
                            movie.duration,
                            movie.summary,
                            movie.rating,
                            genres,
                            str(movie.originallyAvailableAt),
                        ))
                        log_debug(f"🎬 Added new movie: {movie.title} ({movie.year})")
                    except Exception as e:
                        log_debug(f"❌ Error adding movie {movie.title}: {e}")
                else:
                    cur.execute(
                        "UPDATE films SET title=?, year=? WHERE id=?",
                        (movie.title, movie.year, movie.ratingKey)
                    )

        elif section.type == "show":
            for serie in section.all():
                if int(serie.ratingKey) not in existing_series_ids:
                    try:
                        genres = ", ".join([g.tag for g in serie.genres]) if serie.genres else ""
                        series_insert.append((
                            serie.ratingKey,
                            serie.title,
                            serie.summary,
                            serie.rating,
                            genres,
                        ))
                        log_debug(f"📺 Added new series: {serie.title}")
                    except Exception as e:
                        log_debug(f"❌ Error series {serie.title}: {e}")
                else:
                    cur.execute(
                        "UPDATE series SET title=? WHERE id=?",
                        (serie.title, serie.ratingKey)
                    )

                for saison in serie.seasons():
                    if int(saison.ratingKey) not in existing_saison_ids:
                        saison_insert.append((
                            saison.ratingKey, serie.ratingKey, saison.index, saison.summary
                        ))
                        log_debug(f"  ↳ New season {saison.index}")

                    for e in saison.episodes():
                        if int(e.ratingKey) not in existing_episode_ids:
                            try:
                                p = e.media[0].parts[0]
                                episode_insert.append((
                                    e.ratingKey,
                                    saison.ratingKey,
                                    e.index,
                                    e.title,
                                    p.key,
                                    e.duration,
                                    e.summary,
                                    e.rating,
                                ))
                                log_debug(f"    ↳ New episode {e.index}: {e.title}")
                            except Exception as ex:
                                log_debug(f"❌ Error episode {e.title}: {ex}")

    if film_insert:
        cur.executemany(
            "INSERT INTO films VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", film_insert
        )
    if series_insert:
        cur.executemany(
            "INSERT INTO series VALUES (?, ?, ?, ?, ?)", series_insert
        )
    if saison_insert:
        cur.executemany(
            "INSERT INTO saisons VALUES (?, ?, ?, ?)", saison_insert
        )
    if episode_insert:
        cur.executemany(
            "INSERT INTO episodes VALUES (?, ?, ?, ?, ?, ?, ?, ?)", episode_insert
        )

    conn.commit()
    log_debug("✅ Cache updated (only new items added).")
