#!/usr/bin/env python3
import sqlite3, os, sys
from plexapi.server import PlexServer

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
cache_dir = os.path.expanduser("~/.cache/plex-minimal")
db_path = os.path.join(cache_dir, "cache.db")
tmp_db = db_path + ".tmp"

os.makedirs(cache_dir, exist_ok=True)

with open(token_path) as f:
    token = f.read().strip()

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
        except: pass

    for serie in plex.library.section('Séries').all():
        try:
            cur.execute("INSERT INTO series VALUES (?, ?)", (serie.ratingKey, serie.title))
            for saison in serie.seasons():
                cur.execute("INSERT INTO saisons VALUES (?, ?, ?)", (saison.ratingKey, serie.ratingKey, saison.index))
                for e in saison.episodes():
                    try:
                        p = e.media[0].parts[0]
                        cur.execute("INSERT INTO episodes VALUES (?, ?, ?, ?, ?)",
                                    (e.ratingKey, saison.ratingKey, e.index, e.title, p.key))
                    except: pass
        except: pass

    conn.commit()

os.replace(tmp_db, db_path)
