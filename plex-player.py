from plexapi.server import PlexServer
import subprocess
import os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")

with open(token_path) as f:
    token = f.read().strip()

plex = PlexServer(baseurl, token)
movies = plex.library.section('Films').all()

movie_map = {f"{movie.title} ({movie.year})": movie for movie in movies}
choices = "\n".join(movie_map.keys())

fzf = subprocess.run(["fzf", "--prompt=Choisir un film : "], input=choices, text=True, capture_output=True)

if fzf.returncode != 0:
    print("Aucun film sélectionné.")
    exit(0)

selected = fzf.stdout.strip()
media = movie_map[selected]

url = media.getStreamURL()
subprocess.run(["mpv", url])
