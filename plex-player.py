#!/usr/bin/env python3
import sqlite3, subprocess, os, sys

baseurl = 'http://192.168.1.2:32400'
token = open(os.path.expanduser("~/.config/plex-minimal/token")).read().strip()
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")
cache_script = os.environ.get("BUILD_CACHE", "build_cache.py")

if not os.path.exists(db_path):
    subprocess.run(["python3", cache_script], check=True)

conn = sqlite3.connect(db_path)
cur = conn.cursor()

try:
    subprocess.Popen(["python3", cache_script],
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
except:
    pass

def fzf_select(prompt, items):
    fzf = subprocess.run(["fzf", "--prompt=" + prompt], input=items, text=True, capture_output=True)
    return fzf.stdout.strip() if fzf.returncode == 0 else None

def lancer_mpv(titre, url):
    subprocess.Popen([
        "mpv", "--force-window=yes", "--force-seekable=yes", "--hwdec=vaapi",
        "--msg-level=ffmpeg/video=error", f"--title={titre}", url
    ])

def menu_films():
    cur.execute("SELECT title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
    items = [(f"{t} ({y})", k) for t, y, k in cur.fetchall()]
    choix = fzf_select("🎬 Choisir un film : ", "\n".join(t for t, _ in items))
    if not choix: return False
    lancer_mpv(choix, f"{baseurl}{dict(items)[choix]}?X-Plex-Token={token}")
    return True

def menu_series():
    cur.execute("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
    series = cur.fetchall()
    s_id = dict((t, i) for i, t in series).get(fzf_select("📺 Choisir une série : ", "\n".join(t for _, t in series)))
    if not s_id: return False

    cur.execute("SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index", (s_id,))
    saisons = cur.fetchall()
    sa_id = dict((f"Saison {i}", sid) for sid, i in saisons).get(
        fzf_select("📂 Choisir une saison : ", "\n".join(f"Saison {i}" for _, i in saisons)))
    if not sa_id: return False

    cur.execute("SELECT episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", (sa_id,))
    episodes = cur.fetchall()
    e_map = {f"{i:02d}. {t}": k for i, t, k in episodes}
    choix = fzf_select("🎞️ Choisir un épisode : ", "\n".join(e_map))
    if not choix: return False
    lancer_mpv(choix, f"{baseurl}{e_map[choix]}?X-Plex-Token={token}")
    return True

while True:
    sel = fzf_select("🎯 Choisir contenu : ", "Films\nSéries")
    if not sel: break
    if (sel == "Films" and menu_films()) or (sel == "Séries" and menu_series()):
        input()
