import sqlite3, subprocess, os, argparse

CONFIG_DIR = os.path.expanduser("~/.config/plex-minimal")
CACHE_DIR = os.path.expanduser("~/.cache/plex-minimal")
os.makedirs(CONFIG_DIR, exist_ok=True)
os.makedirs(CACHE_DIR, exist_ok=True)
TOKEN_PATH = os.path.join(CONFIG_DIR, "token")
BASEURL_PATH = os.path.join(CONFIG_DIR, "baseurl")
DB_PATH = os.path.join(CACHE_DIR, "cache.db")
CACHE_SCRIPT = os.environ.get("BUILD_CACHE", "build_cache.py")

def save_config(path, value):
    with open(path, "w") as f:
        f.write(value.strip())

def load_config(path, missing_msg=None):
    try:
        with open(path) as f:
            return f.read().strip()
    except FileNotFoundError:
        if missing_msg:
            print(missing_msg)
        return None

def parse_args():
    parser = argparse.ArgumentParser(description="Minimal Plex client with MPV")
    parser.add_argument("--baseurl", help="Plex server URL (e.g. http://192.168.1.2:32400)")
    parser.add_argument("--token", help="Plex authentication token")
    parser.add_argument("--debug", action="store_true", help="Show detailed logs")
    return parser.parse_args()

args = parse_args()

def log_debug(msg):
    if args.debug:
        print(f"[DEBUG] {msg}")

if args.baseurl:
    save_config(BASEURL_PATH, args.baseurl)
if args.token:
    save_config(TOKEN_PATH, args.token)

baseurl = load_config(BASEURL_PATH, "❌ Missing baseurl. Use --baseurl to save it.")
token = load_config(TOKEN_PATH, "❌ Missing token. Use --token to save it.")
if not baseurl or not token:
    exit(1)

if not os.path.exists(DB_PATH):
    log_debug("Database missing. Generating...")
    subprocess.run(["python3", CACHE_SCRIPT], check=True)

conn = sqlite3.connect(DB_PATH)
cur = conn.cursor()

subprocess.Popen(["python3", CACHE_SCRIPT], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
log_debug("Cache update started in background.")

def fzf_select(prompt, items, default_first=False):
    options = ["fzf", "--prompt=" + prompt]
    if default_first:
        options += ["--header-lines=1"]
        items = [""] + items
    result = subprocess.run(options, input="\n".join(items), text=True, capture_output=True)
    return result.stdout.strip() if result.returncode == 0 else None

def lancer_mpv(title, url):
    log_debug(f"Launching MPV: {title} - {url}")
    subprocess.run([
        "mpv", "--force-window=yes", "--hwdec=vaapi",
        "--fullscreen",
        f"--title={title}", f"{url}?X-Plex-Token={token}"
    ])

def menu_films():
    cur.execute("SELECT title, year, part_key FROM films ORDER BY title COLLATE NOCASE")
    items = [(f"{t} ({y})", k) for t, y, k in cur.fetchall()]
    choice = fzf_select("🎬 Movie: ", [i[0] for i in items])
    if choice:
        lancer_mpv(choice, baseurl + dict(items)[choice])
        return True
    return False

def menu_series():
    cur.execute("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
    series = cur.fetchall()
    title = fzf_select("📺 Series: ", [t for _, t in series])
    if not title:
        return False

    s_id = dict((t, i) for i, t in series).get(title)
    if not s_id:
        print(f"Series not found: {title}")
        return False

    cur.execute("SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index", (s_id,))
    seasons = cur.fetchall()
    label = fzf_select("📂 Season: ", [f"Season {i}" for _, i in seasons])
    if not label:
        return False
    sa_id = dict((f"Season {i}", sid) for sid, i in seasons)[label]

    cur.execute("SELECT episode_index, title, part_key FROM episodes WHERE saison_id = ? ORDER BY episode_index", (sa_id,))
    episodes = cur.fetchall()
    e_map = [(f"{i:02d}. {t}", k) for i, t, k in episodes]

    choice = fzf_select("🎞️ Episode: ", [label for label, _ in e_map])
    if not choice:
        return False

    index = next((i for i, (label, _) in enumerate(e_map) if label == choice), None)
    if index is None:
        print("Selected episode not found.")
        return False

    while 0 <= index < len(e_map):
        label, part_key = e_map[index]
        print(f"Playing: {label}")
        lancer_mpv(label, baseurl + part_key)

        prev_label = f"⏮️ Previous: {e_map[index - 1][0]}" if index > 0 else None
        next_label = f"⏭️ Next: {e_map[index + 1][0]}" if index < len(e_map) - 1 else None

        options = []
        if next_label: options.append(next_label)
        if prev_label: options.append(prev_label)
        options.append("❌ Quit")

        next_action = fzf_select("▶️ Choose action: ", options, default_first=True)

        if next_action and next_action.startswith("⏮️"):
            index = max(0, index - 1)
        elif next_action and next_action.startswith("⏭️"):
            index += 1
        else:
            break

    return True

while True:
    sel = fzf_select("🎯 Choose: ", ["Movies", "Series"])
    if not sel: break
    if sel == "Movies":
        menu_films()
    elif sel == "Series":
        menu_series()
