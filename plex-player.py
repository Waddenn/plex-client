#!/usr/bin/env python3
import sqlite3
import subprocess
import os
import sys

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")
cache_script = os.environ.get("BUILD_CACHE", "build_cache.py")

# Lire le token
with open(token_path) as f:
    token = f.read().strip()

# --refresh : met à jour la base
if "--refresh" in sys.argv:
    print("🔄 Rafraîchissement du cache Plex...")
    subprocess.run(["python3", cache_script], check=True)

# Connexion à la DB
conn = sqlite3.connect(db_path)
cur = conn.cursor()

def lancer_mpv(titre, url):
    print(f"Lancement de {titre} en lecture directe...")
    subprocess.Popen([
        "mpv",
        "--force-window=yes",
        "--force-seekable=yes",
        "--hwdec=vaapi",
        "--msg-level=ffmpeg/video=error",
        f"--title={titre}",
        url
    ])

def menu_films():
    cur.execute("SELECT title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
    movies = cur.fetchall()
    movie_map = {f"{title} ({year})": part_key for (title, year, part_key) in movies}
    choices = "\n".join(movie_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=🎬 Choisir un film : "], input=choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    selected = fzf.stdout.strip()
    part_key = movie_map[selected]
    url = f"{baseurl}{part_key}?X-Plex-Token={token}"
    lancer_mpv(selected, url)
    return True

def menu_series():
    cur.execute("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
    series = cur.fetchall()
    serie_map = {title: id for id, title in series}
    choices = "\n".join(serie_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=📺 Choisir une série : "], input=choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    serie_title = fzf.stdout.strip()
    serie_id = serie_map[serie_title]

    cur.execute("SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index", (serie_id,))
    saisons = cur.fetchall()
    saison_map = {f"Saison {index}": id for id, index in saisons}
    saison_choices = "\n".join(saison_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=📂 Choisir une saison : "], input=saison_choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    saison_id = saison_map[fzf.stdout.strip()]

    cur.execute("SELECT episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", (saison_id,))
    episodes = cur.fetchall()
    episode_map = {f"{index:02d}. {title}": part_key for index, title, part_key in episodes}
    episode_choices = "\n".join(episode_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=🎞️ Choisir un épisode : "], input=episode_choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    selected = fzf.stdout.strip()
    part_key = episode_map[selected]
    url = f"{baseurl}{part_key}?X-Plex-Token={token}"
    lancer_mpv(selected, url)
    return True

# === Menu principal ===
while True:
    choix = subprocess.run(["fzf", "--prompt=🎯 Choisir contenu : ", "--height=10%"], input="Films\nSéries", text=True, capture_output=True)
    if choix.returncode != 0:
        print("Fermeture du programme.")
        break

    lancer = False
    if choix.stdout.strip() == "Films":
        lancer = menu_films()
    elif choix.stdout.strip() == "Séries":
        lancer = menu_series()

    if lancer:
        input("Appuie sur Entrée pour revenir au menu principal, ou Ctrl+C pour quitter.")
