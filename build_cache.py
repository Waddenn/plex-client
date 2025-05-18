#!/usr/bin/env python3
import sqlite3, os
from plexapi.server import PlexServer
import argparse

# === CONFIGURATION ===
config_dir = os.path.expanduser("~/.config/plex-minimal")
os.makedirs(config_dir, exist_ok=True)
token_path = os.path.join(config_dir, "token")
baseurl_path = os.path.join(config_dir, "baseurl")
cache_dir = os.path.expanduser("~/.cache/plex-minimal")
os.makedirs(cache_dir, exist_ok=True)
db_path = os.path.join(cache_dir, "cache.db")
tmp_db = db_path + ".tmp"

# === ARGUMENTS ===
parser = argparse.ArgumentParser(description="Génère la base Plex minimale")
parser.add_argument("--baseurl", help="URL du serveur Plex")
parser.add_argument("--token", help="Token d'authentification Plex")
args = parser.parse_args()

# Enregistrement si fourni
if args.baseurl:
    with open(baseurl_path, "w") as f:
        f.write(args.baseurl.strip())

if args.token:
    with open(token_path, "w") as f:
        f.write(args.token.strip())

# Lecture
try:
    with open(baseurl_path) as f:
        baseurl = f.read().strip()
except FileNotFoundError:
    print("❌ baseurl manquant.")
    exit(1)

try:
    with open(token_path) as f:
        token = f.read().strip()
except FileNotFoundError:
    print("❌ token manquant.")
    exit(1)

# === CONNEXION ET GÉNÉRATION ===
plex = PlexServer(baseurl, token)

with sqlite3.connect(tmp_db) as conn:
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
        except:
            pass

    for serie in plex.library.section('Séries').all():
        try:
            cur.execute("INSERT INTO series VALUES (?, ?)", (serie.ratingKey, serie.title))
            for saison in serie.seasons():
                cur.execute("INSERT INTO saisons VALUES (?, ?, ?)", (saison.ratingKey, serie.ratingKey, saison.index))
                for e in saison.episodes():
                    try:
                        p = e.media[0].parts[0]
                        cur.execute("INSERT INTO episodes VALUES (?, ?, ?, ?, ?)", (
                            e.ratingKey, saison.ratingKey, e.index, e.title, p.key
                        ))
                    except:
                        pass
        except:
            pass

    conn.commit()

os.replace(tmp_db, db_path)
