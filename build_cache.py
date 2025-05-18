#!/usr/bin/env python3
import sqlite3
from plexapi.server import PlexServer
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")

with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)
os.makedirs(os.path.dirname(db_path), exist_ok=True)

conn = sqlite3.connect(db_path)
cur = conn.cursor()
cur.execute("PRAGMA journal_mode=WAL;")

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

with conn:
    for table in ("films", "series", "saisons", "episodes"):
        cur.execute(f"DELETE FROM {table}")

    for movie in plex.library.section('Films').all():
        try:
            p = movie.media[0].parts[0]
            cur.execute("INSERT INTO films VALUES (?, ?, ?, ?)",
                        (movie.ratingKey, movie.title, movie.year, p.key))
        except: pass

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
                    except: pass
        except: pass

conn.close()
