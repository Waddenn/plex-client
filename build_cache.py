#!/usr/bin/env python3
import sqlite3
from plexapi.server import PlexServer
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")

# Lire le token Plex
with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)

# Créer le dossier de cache si nécessaire
os.makedirs(os.path.dirname(db_path), exist_ok=True)

# Connexion à SQLite
conn = sqlite3.connect(db_path)
cur = conn.cursor()

# Création des tables
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

# Suppression des anciennes données
cur.execute("DELETE FROM films")
cur.execute("DELETE FROM series")
cur.execute("DELETE FROM saisons")
cur.execute("DELETE FROM episodes")
conn.commit()

# 🟦 Récupération des films
print("📦 Mise en cache des films...")
for movie in plex.library.section('Films').all():
    try:
        part = movie.media[0].parts[0]
        cur.execute(
            "INSERT INTO films (id, title, year, part_key) VALUES (?, ?, ?, ?)",
            (movie.ratingKey, movie.title, movie.year, part.key)
        )
    except Exception as e:
        print(f"⛔ Erreur film {movie.title}: {e}")

# 🟨 Récupération des séries
print("📦 Mise en cache des séries...")
for serie in plex.library.section('Séries').all():
    try:
        cur.execute("INSERT INTO series (id, title) VALUES (?, ?)", (serie.ratingKey, serie.title))
        for saison in serie.seasons():
            cur.execute(
                "INSERT INTO saisons (id, serie_id, saison_index) VALUES (?, ?, ?)",
                (saison.ratingKey, serie.ratingKey, saison.index)
            )
            for episode in saison.episodes():
                try:
                    part = episode.media[0].parts[0]
                    cur.execute(
                        "INSERT INTO episodes (id, saison_id, episode_index, title, part_key) VALUES (?, ?, ?, ?, ?)",
                        (episode.ratingKey, saison.ratingKey, episode.index, episode.title, part.key)
                    )
                except Exception as ep:
                    print(f"⛔ Erreur épisode {episode.title}: {ep}")
    except Exception as s:
        print(f"⛔ Erreur série {serie.title}: {s}")

conn.commit()
conn.close()

print("✅ Cache SQLite Plex généré avec succès.")
