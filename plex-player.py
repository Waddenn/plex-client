#!/usr/bin/env python3
from plexapi.server import PlexServer
import subprocess
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")

with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)
movies = plex.library.section('Films').all()

# Index des films
movie_map = {f"{movie.title} ({movie.year})": movie for movie in movies}
choices = "\n".join(movie_map.keys())

while True:
    # Choix via fzf
    fzf = subprocess.run(["fzf", "--prompt=Choisir un film : "], input=choices, text=True, capture_output=True)

    if fzf.returncode != 0:
        print("Fermeture du programme.")
        break

    selected = fzf.stdout.strip()
    media = movie_map[selected]

    url = media.getStreamURL()

    # Lancement non bloquant
    print(f"Lancement de {selected}...")
    subprocess.Popen(["mpv", url])

    # Petite pause ou message pour continuer
    input("Appuie sur Entrée pour sélectionner un autre film, ou Ctrl+C pour quitter.")
