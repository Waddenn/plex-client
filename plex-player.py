#!/usr/bin/env python3
import sqlite3, subprocess, os

baseurl = 'http://192.168.1.2:32400'
token_path = os.path.expanduser("~/.config/plex-minimal/token")
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")
cache_script = os.environ.get("BUILD_CACHE", "build_cache.py")

with open(token_path) as f:
    token = f.read().strip()

# Regénère la base si absente
if not os.path.exists(db_path):
    subprocess.run(["python3", cache_script], check=True)

# Connexion à la base
conn = sqlite3.connect(db_path)
cur = conn.cursor()

# Relance le script de cache en arrière-plan
subprocess.Popen(["python3", cache_script], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def fzf_select(prompt, items):
    result = subprocess.run(["fzf", "--prompt=" + prompt], input="\n".join(items), text=True, capture_output=True)
    return result.stdout.strip() if result.returncode == 0 else None

def lancer_mpv(titre, url):
    subprocess.Popen([
        "mpv", "--force-window=yes", "--hwdec=vaapi",
        f"--title={titre}", f"{url}?X-Plex-Token={token}"
    ])

def menu_films():
    cur.execute("SELECT title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
    items = [(f"{t} ({y})", k) for t, y, k in cur.fetchall()]
    choix = fzf_select("🎬 Film : ", [i[0] for i in items])
    if choix:
        lancer_mpv(choix, baseurl + dict(items)[choix])
        return True
    return False

def menu_series():
    cur.execute("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
    series = cur.fetchall()
    titre = fzf_select("📺 Série : ", [t for _, t in series])
    if not titre:
        return False

    s_id = dict((t, i) for i, t in series).get(titre)
    if not s_id:
        print(f"Série introuvable : {titre}")
        return False

    cur.execute("SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index", (s_id,))
    saisons = cur.fetchall()
    label = fzf_select("📂 Saison : ", [f"Saison {i}" for _, i in saisons])
    if not label:
        return False
    sa_id = dict((f"Saison {i}", sid) for sid, i in saisons)[label]

    cur.execute("SELECT episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", (sa_id,))
    e_map = {f"{i:02d}. {t}": k for i, t, k in cur.fetchall()}
    choix = fzf_select("🎞️ Épisode : ", list(e_map))
    if choix:
        lancer_mpv(choix, baseurl + e_map[choix])
        return True
    return False


while True:
    sel = fzf_select("🎯 Choisir : ", ["Films", "Séries"])
    if not sel: break
    if sel == "Films": menu_films()
    elif sel == "Séries": menu_series()
