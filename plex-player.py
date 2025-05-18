#!/usr/bin/env python3
import sqlite3, subprocess, os, argparse

# === CONFIGURATION ===
config_dir = os.path.expanduser("~/.config/plex-minimal")
os.makedirs(config_dir, exist_ok=True)
token_path = os.path.join(config_dir, "token")
baseurl_path = os.path.join(config_dir, "baseurl")
db_path = os.path.expanduser("~/.cache/plex-minimal/cache.db")
cache_script = os.environ.get("BUILD_CACHE", "build_cache.py")

# === ARGUMENTS ===
parser = argparse.ArgumentParser(description="Client Plex minimal avec MPV")
parser.add_argument("--baseurl", help="URL du serveur Plex (ex: http://192.168.1.2:32400)")
parser.add_argument("--token", help="Token Plex d'authentification")
args = parser.parse_args()

# Enregistrement des arguments si fournis
if args.baseurl:
    with open(baseurl_path, "w") as f:
        f.write(args.baseurl.strip())

if args.token:
    with open(token_path, "w") as f:
        f.write(args.token.strip())

# Lecture de la configuration
try:
    with open(baseurl_path) as f:
        baseurl = f.read().strip()
except FileNotFoundError:
    print("❌ baseurl manquant. Utilisez --baseurl pour l’enregistrer.")
    exit(1)

try:
    with open(token_path) as f:
        token = f.read().strip()
except FileNotFoundError:
    print("❌ token manquant. Utilisez --token pour l’enregistrer.")
    exit(1)

# === INITIALISATION DB ===
if not os.path.exists(db_path):
    subprocess.run(["python3", cache_script], check=True)

conn = sqlite3.connect(db_path)
cur = conn.cursor()

# Lancement script cache en fond
subprocess.Popen(["python3", cache_script], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

# === UTILITAIRES ===
def fzf_select(prompt, items, default_first=False):
    options = ["fzf", "--prompt=" + prompt]
    if default_first:
        options += ["--header-lines=1"]
        items = [""] + items
    result = subprocess.run(options, input="\n".join(items), text=True, capture_output=True)
    return result.stdout.strip() if result.returncode == 0 else None

def lancer_mpv(titre, url):
    subprocess.run([
        "mpv", "--force-window=yes", "--hwdec=vaapi",
        "--fullscreen",
        f"--title={titre}", f"{url}?X-Plex-Token={token}"
    ])

# === FILMS ===
def menu_films():
    cur.execute("SELECT title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
    items = [(f"{t} ({y})", k) for t, y, k in cur.fetchall()]
    choix = fzf_select("🎬 Film : ", [i[0] for i in items])
    if choix:
        lancer_mpv(choix, baseurl + dict(items)[choix])
        return True
    return False

# === SÉRIES ===
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
    episodes = cur.fetchall()
    e_map = [(f"{i:02d}. {t}", k) for i, t, k in episodes]

    choix = fzf_select("🎞️ Épisode : ", [label for label, _ in e_map])
    if not choix:
        return False

    index = next((i for i, (label, _) in enumerate(e_map) if label == choix), None)
    if index is None:
        print("Épisode sélectionné introuvable.")
        return False

    while 0 <= index < len(e_map):
        label, part_key = e_map[index]
        print(f"Lecture : {label}")
        lancer_mpv(label, baseurl + part_key)

        # Définir les labels des épisodes voisins
        prev_label = f"⏮️ Précédent : {e_map[index - 1][0]}" if index > 0 else None
        next_label = f"⏭️ Suivant : {e_map[index + 1][0]}" if index < len(e_map) - 1 else None

        options = []
        if next_label: options.append(next_label)
        if prev_label: options.append(prev_label)
        options.append("❌ Quitter")

        next_action = fzf_select(
            "▶️ Choix de l'action : ",
            options,
            default_first=True
        )

        if next_action and next_action.startswith("⏮️"):
            index = max(0, index - 1)
        elif next_action and next_action.startswith("⏭️"):
            index += 1
        else:
            break

    return True

# === MENU PRINCIPAL ===
while True:
    sel = fzf_select("🎯 Choisir : ", ["Films", "Séries"])
    if not sel: break
    if sel == "Films":
        menu_films()
    elif sel == "Séries":
        menu_series()
