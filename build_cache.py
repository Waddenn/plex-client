#!/usr/bin/env python3
import sqlite3
from plexapi.server import PlexServer
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
cache_dir = os.path.expanduser("~/.cache/plex-minimal")
db_path = os.path.join(cache_dir, "cache.db")
tmp_db = db_path + ".tmp"

# Ensure the cache directory exists
os.makedirs(cache_dir, exist_ok=True)

# Read the token
with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)

# Connect to the temporary database
conn = sqlite3.connect(tmp_db)
cur = conn.cursor()

# Enable WAL mode for better concurrency
cur.execute("PRAGMA journal_mode=WAL;")

# Create tables if they don't exist
cur.executescript('''
CREATE TABLE IF NOT EXISTS films (
    id INTEGER PRIMARY KEY,
    title TEXT,
    year INTEGER,
    part_key TEXT
);
CREATE TABLE IF NOT EXISTS series (
    id INTEGER PRIMARY KEY,
    title TEXT
);
CREATE TABLE IF NOT EXISTS saisons (
    id INTEGER PRIMARY KEY,
    serie_id INTEGER,
    saison_index INTEGER
);
CREATE TABLE IF NOT EXISTS episodes (
    id INTEGER PRIMARY KEY,
    saison_id INTEGER,
    episode_index INTEGER,
    title TEXT,
    part_key TEXT
);
''')

# Insert data from Plex into the database
try:
    # Insert films
    for movie in plex.library.section('Films').all():
        try:
            p = movie.media[0].parts[0]
            cur.execute("INSERT INTO films VALUES (?, ?, ?, ?)",
                        (movie.ratingKey, movie.title, movie.year, p.key))
        except Exception as e:
            print(f"Error inserting movie: {e}")
    
    # Insert series
    for serie in plex.library.section('Séries').all():
        try:
            cur.execute("INSERT INTO series VALUES (?, ?)", (serie.ratingKey, serie.title))
            for saison in serie.seasons():
                cur.execute("INSERT INTO saisons VALUES (?, ?, ?)",
                            (saison.ratingKey, serie.ratingKey, saison.index))
                for e in saison.episodes():
                    try:
                        p = e.media[0].parts[0]
                        cur.execute("INSERT INTO episodes VALUES (?, ?, ?, ?, ?)",
                                    (e.ratingKey, saison.ratingKey, e.index, e.title, p.key))
                    except Exception as e:
                        print(f"Error inserting episode: {e}")
        except Exception as e:
            print(f"Error inserting series: {e}")
    
    conn.commit()
except Exception as e:
    print(f"Error during data insertion: {e}")
finally:
    conn.close()

# Ensure the temporary database file is replaced only if the temporary database exists
if os.path.exists(tmp_db):
    try:
        os.replace(tmp_db, db_path)
        print(f"Replaced {tmp_db} with {db_path}")
    except Exception as e:
        print(f"Error replacing database: {e}")
else:
    print(f"Temporary database file {tmp_db} does not exist.")
