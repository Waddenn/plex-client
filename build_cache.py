import sqlite3, os
from plexapi.server import PlexServer
import argparse

CONFIG_DIR = os.path.expanduser("~/.config/plex-minimal")
CACHE_DIR = os.path.expanduser("~/.cache/plex-minimal")
os.makedirs(CONFIG_DIR, exist_ok=True)
os.makedirs(CACHE_DIR, exist_ok=True)
TOKEN_PATH = os.path.join(CONFIG_DIR, "token")
BASEURL_PATH = os.path.join(CONFIG_DIR, "baseurl")
DB_PATH = os.path.join(CACHE_DIR, "cache.db")
TMP_DB = DB_PATH + ".tmp"

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

with sqlite3.connect(TMP_DB) as conn:
    cur = conn.cursor()
    cur.executescript('''
        CREATE TABLE IF NOT EXISTS films (id INTEGER PRIMARY KEY, title TEXT, year INTEGER, part_key TEXT);
        CREATE TABLE IF NOT EXISTS series (id INTEGER PRIMARY KEY, title TEXT);
        CREATE TABLE IF NOT EXISTS saisons (id INTEGER PRIMARY KEY, serie_id INTEGER, saison_index INTEGER);
        CREATE TABLE IF NOT EXISTS episodes (id INTEGER PRIMARY KEY, saison_id INTEGER, episode_index INTEGER, title TEXT, part_key TEXT);
    ''')

    for movie in plex.library.section('Films').all():
        try:
            p = movie.media[0].parts[0]
            cur.execute("INSERT INTO films VALUES (?, ?, ?, ?)", (movie.ratingKey, movie.title, movie.year, p.key))
            log_debug(f"Added movie: {movie.title} ({movie.year})")
        except Exception as e:
            log_debug(f"❌ Error adding movie {movie.title}: {e}")

    for serie in plex.library.section('Séries').all():
        try:
            cur.execute("INSERT INTO series VALUES (?, ?)", (serie.ratingKey, serie.title))
            log_debug(f"Added series: {serie.title}")
            for saison in serie.seasons():
                cur.execute("INSERT INTO saisons VALUES (?, ?, ?)", (saison.ratingKey, serie.ratingKey, saison.index))
                log_debug(f"  ↳ Season {saison.index}")
                for e in saison.episodes():
                    try:
                        p = e.media[0].parts[0]
                        cur.execute("INSERT INTO episodes VALUES (?, ?, ?, ?, ?)", (
                            e.ratingKey, saison.ratingKey, e.index, e.title, p.key
                        ))
                        log_debug(f"    ↳ Episode {e.index}: {e.title}")
                    except Exception as ex:
                        log_debug(f"❌ Error episode {e.title}: {ex}")
        except Exception as e:
            log_debug(f"❌ Error series {serie.title}: {e}")

    conn.commit()

os.replace(TMP_DB, DB_PATH)
log_debug("Database updated.")
