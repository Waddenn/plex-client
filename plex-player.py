#!/usr/bin/env python3
from plexapi.server import PlexServer
import subprocess
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")

with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)
films = plex.library.section('Films').all()
series = plex.library.section('Séries').all()

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
    movie_map = {f"{movie.title} ({movie.year})": movie for movie in films}
    choices = "\n".join(movie_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=🎬 Choisir un film : "], input=choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    selected = fzf.stdout.strip()
    movie = movie_map[selected]
    part = movie.media[0].parts[0]
    url = f"{baseurl}{part.key}?X-Plex-Token={token}"
    lancer_mpv(selected, url)
    return True


def menu_series():
    serie_map = {serie.title: serie for serie in series}
    choices = "\n".join(serie_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=📺 Choisir une série : "], input=choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    serie = serie_map[fzf.stdout.strip()]
    saison_map = {f"Saison {s.index}": s for s in serie.seasons()}
    saison_choices = "\n".join(saison_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=📂 Choisir une saison : "], input=saison_choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    saison = saison_map[fzf.stdout.strip()]
    episodes = saison.episodes()
    episode_map = {f"{e.index:02d}. {e.title}": e for e in episodes}
    episode_choices = "\n".join(episode_map.keys())

    fzf = subprocess.run(["fzf", "--prompt=🎞️ Choisir un épisode : "], input=episode_choices, text=True, capture_output=True)
    if fzf.returncode != 0:
        return False

    episode = episode_map[fzf.stdout.strip()]
    part = episode.media[0].parts[0]
    url = f"{baseurl}{part.key}?X-Plex-Token={token}"
    lancer_mpv(f"{serie.title} - {episode.title}", url)
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
