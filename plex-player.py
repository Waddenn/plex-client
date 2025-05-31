#!/usr/bin/env python3
import sqlite3, subprocess, os, argparse, requests
from xml.etree import ElementTree

CONFIG_DIR = os.path.expanduser("~/.config/plex-minimal")
CACHE_DIR = os.path.expanduser("~/.cache/plex-minimal")
os.makedirs(CONFIG_DIR, exist_ok=True)
os.makedirs(CACHE_DIR, exist_ok=True)
TOKEN_PATH = os.path.join(CONFIG_DIR, "token")
BASEURL_PATH = os.path.join(CONFIG_DIR, "baseurl")
DB_PATH = os.path.join(CACHE_DIR, "cache.db")
CACHE_SCRIPT = os.environ.get("BUILD_CACHE", "build_cache.py")


def save_config(path, value):
    """Save a configuration value to a file."""
    with open(path, "w") as f:
        f.write(value.strip())


def load_config(path, missing_msg=None):
    """Load a configuration value from a file, optionally printing a message if missing."""
    try:
        with open(path) as f:
            return f.read().strip()
    except FileNotFoundError:
        if missing_msg:
            print(missing_msg)
        return None


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description="Minimal Plex client with MPV")
    parser.add_argument("--baseurl", help="Plex server URL")
    parser.add_argument("--token", help="Plex authentication token")
    parser.add_argument("--debug", action="store_true", help="Show debug output")
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

subprocess.Popen(
    ["python3", CACHE_SCRIPT], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
)
log_debug("Cache update started in background.")


def fzf_select(prompt, items, default_first=False, preview_cmd=None):
    """Run fzf with the given prompt and items, optionally with a preview command."""
    options = ["fzf", "--prompt=" + prompt, "--preview-window=right:60%:wrap"]
    if default_first:
        options += ["--header-lines=1"]
        items = [""] + items
    if preview_cmd:
        options += ["--preview", preview_cmd]
    result = subprocess.run(
        options, input="\n".join(items), text=True, capture_output=True
    )
    return result.stdout.strip() if result.returncode == 0 else None

def launch_mpv(title, url):
    """Launch MPV player with the given title and URL."""
    log_debug(f"Launching MPV: {title} - {url}")
    mpv_args = os.environ.get("MPV_CONFIG_OVERRIDE", "").split()
    subprocess.run(
        [
            "mpv",
            *mpv_args,
            "--force-window=yes",
            "--hwdec=vaapi",
            "--fullscreen",
            "--alang=eng",
            "--slang=eng",
            "--vo=gpu",
            "--gpu-api=opengl",
            f"--title={title}",
            f"{url}?X-Plex-Token={token}",
        ]
    )


def run_fzf(items, prompt, preview_script=None, expect_keys=None, default_first=False):
    """Run fzf with options and return the pressed key and selected choice."""
    options = [
        "fzf",
        "--prompt=" + prompt,
        "--preview-window=right:60%:wrap"
    ]
    if preview_script:
        options += ["--preview", preview_script]
    if expect_keys:
        options += [f"--expect={expect_keys}"]
    if default_first:
        options += ["--header-lines=1"]
        items = [""] + items
    result = subprocess.run(
        options, input="\n".join(items), text=True, capture_output=True
    )
    output = result.stdout.splitlines()
    key = output[0] if output else ""
    choice = output[1] if len(output) > 1 else None
    return key, choice

def get_film_items(sort_mode, sort_order, cur):
    """Retrieve film items from the database, sorted by the given mode and order."""
    sort_options = {
        "title": ("title", "COLLATE NOCASE", "ASC"),
        "rating": ("rating", "", "DESC"),
        "year": ("year", "", "DESC")
    }
    key, extra, default_order = sort_options[sort_mode]
    order = "ASC" if sort_order == "asc" else "DESC"
    if sort_mode == "title":
        sql = f"SELECT title, year FROM films ORDER BY {key} {extra} {order}"
    else:
        sql = f"SELECT title, year FROM films ORDER BY {key} {order}, title COLLATE NOCASE"
    cur.execute(sql)
    films = cur.fetchall()
    return [(f"{title} ({year})", title) for title, year in films]

def get_preview_script_film():
    """Return a preview script for films."""
    return f"""
python3 -c '
import sqlite3, sys, textwrap
db = "{DB_PATH}"
title = sys.argv[1].rsplit(" (", 1)[0]
conn = sqlite3.connect(db)
cur = conn.cursor()
cur.execute("SELECT title, year, duration, summary, rating, genres, originallyAvailableAt FROM films WHERE title = ?", (title,))
row = cur.fetchone()
if row:
    print(f"🎬 {{row[0]}} ({{row[1]}})\\n")
    print(f"🕒 Duration: {{int(row[2]/60000)}} min")
    print(f"⭐ Rating: {{row[4]}}")
    print(f"🎭 Genres: {{row[5]}}")
    print(f"📅 Release date: {{row[6][:10]}}\\n")
    print("🧾 Synopsis:")
    print("─" * 72)
    wrapped = textwrap.wrap(row[3] or "", width=72)
    for line in wrapped[:15]:
        print("  " + line)
    if len(wrapped) > 15:
        print("  [...]")
else:
    print("No metadata found.")
' {{}}""".strip()

def menu_films():
    """Display the movies menu with sorting options."""
    sort_options = {
        "title": {
            "label": "Title",
            "shortcut": "ctrl-t",
            "default_order": "asc"
        },
        "rating": {
            "label": "Rating",
            "shortcut": "ctrl-r",
            "default_order": "desc"
        },
        "year": {
            "label": "Year",
            "shortcut": "ctrl-y",
            "default_order": "desc"
        }
    }
    shortcut_to_mode = {v["shortcut"]: k for k, v in sort_options.items()}
    sort_mode = "title"
    sort_order = sort_options[sort_mode]["default_order"]

    while True:
        items = get_film_items(sort_mode, sort_order, cur)
        preview_script = get_preview_script_film()
        arrow = "▲" if sort_order == "asc" else "▼"
        prompt = f"🎬 Movie [{sort_options[sort_mode]['label']} {arrow}]: "
        expect_keys = ",".join([v["shortcut"] for v in sort_options.values()])
        key_pressed, choice = run_fzf(
            [i[0] for i in items], prompt, preview_script, expect_keys
        )

        if key_pressed in shortcut_to_mode:
            new_mode = shortcut_to_mode[key_pressed]
            if sort_mode == new_mode:
                sort_order = "desc" if sort_order == "asc" else "asc"
            else:
                sort_mode = new_mode
                sort_order = sort_options[sort_mode]["default_order"]
            continue

        if not choice or choice not in dict(items):
            return
        selected_title = dict(items)[choice]
        cur.execute("SELECT part_key FROM films WHERE title = ?", (selected_title,))
        row = cur.fetchone()
        if not row:
            print("Selected movie not found.")
            continue
        launch_mpv(choice, baseurl + row[0])

def get_preview_script_series():
    """Return a preview script for series."""
    return f"""
python3 -c '
import sqlite3, sys, textwrap
db = "{DB_PATH}"
title = sys.argv[1]
conn = sqlite3.connect(db)
cur = conn.cursor()
cur.execute("SELECT title, summary, rating, genres FROM series WHERE title = ?", (title,))
row = cur.fetchone()
if row:
    print(f"📺 {{row[0]}}\\n")
    print(f"⭐ Rating: {{row[2]}}")
    print(f"🎭 Genres: {{row[3]}}\\n")
    print("🧾 Synopsis:")
    print("─" * 72)
    wrapped = textwrap.wrap(row[1] or "", width=72)
    for line in wrapped[:15]:
        print("  " + line)
    if len(wrapped) > 15:
        print("  [...]")
else:
    print("No metadata found.")
' {{}}""".strip()

def get_preview_script_episode(season_id):
    """Return a preview script for episodes."""
    return f"""
python3 -c '
import sqlite3, sys, textwrap
db = "{DB_PATH}"
ep = sys.argv[1]
idx = int(ep.split(".")[0])
conn = sqlite3.connect(db)
cur = conn.cursor()
cur.execute("SELECT title, duration, summary, rating FROM episodes WHERE episode_index = ? AND saison_id = ?", (idx, {season_id}))
row = cur.fetchone()
if row:
    print(f"🎞️ {{row[0]}}\\n")
    print(f"🕒 Duration: {{int(row[1]/60000)}} min")
    print(f"⭐ Rating: {{row[3]}}\\n")
    print("🧾 Synopsis:")
    print("─" * 72)
    wrapped = textwrap.wrap(row[2] or "", width=72)
    for line in wrapped[:15]:
        print("  " + line)
    if len(wrapped) > 15:
        print("  [...]")
else:
    print("No metadata found.")
' {{}}""".strip()

def get_video_sections(baseurl, token):
    """Return all video section ids (films, séries) from Plex."""
    url = f"{baseurl}/library/sections?X-Plex-Token={token}"
    try:
        r = requests.get(url, timeout=5)
        r.raise_for_status()
        root = ElementTree.fromstring(r.content)
        return [
            directory.attrib["key"]
            for directory in root.findall(".//Directory")
            if directory.attrib.get("type") in ("movie", "show")
        ]
    except Exception as e:
        log_debug(f"Failed to fetch video sections: {e}")
        return []

def get_continue_watching(baseurl, token):
    """Fetch 'Continue Watching' (onDeck) items from Plex."""
    items = []
    section_ids = get_video_sections(baseurl, token)
    for section_id in section_ids:
        url = f"{baseurl}/library/sections/{section_id}/onDeck?X-Plex-Token={token}"
        try:
            r = requests.get(url, timeout=5)
            r.raise_for_status()
            root = ElementTree.fromstring(r.content)
            for video in root.findall(".//Video"):
                title = video.attrib.get("title")
                type_ = video.attrib.get("type")
                year = video.attrib.get("year", "")
                view_offset = int(video.attrib.get("viewOffset", "0"))
                duration = int(video.attrib.get("duration", "0"))
                percent = int((view_offset / duration) * 100) if duration else 0
                part = video.find("Media/Part")
                part_key = part.attrib.get("key") if part is not None else None
                summary = video.attrib.get("summary", "")
                grandparent = video.attrib.get("grandparentTitle", "")
                if type_ == "episode" and grandparent:
                    label = f"{grandparent} - {title} [{percent}%]"
                else:
                    label = f"{title} ({year}) [{type_}] - {percent}% watched"
                items.append({
                    "label": label,
                    "title": title,
                    "part_key": part_key,
                    "summary": summary,
                    "type": type_,
                })
        except Exception as e:
            log_debug(f"Failed to fetch onDeck for section {section_id}: {e}")
    return items

def menu_continue_watching():
    """Display and handle 'Continue Watching' menu."""
    items = get_continue_watching(baseurl, token)
    if not items:
        print("No items in Continue Watching.")
        return
    labels = [item["label"] for item in items]
    choice = fzf_select("⏯️ Continue Watching: ", labels)
    if not choice:
        return
    selected = next((item for item in items if item["label"] == choice), None)
    if not selected:
        return
    url = baseurl + selected["part_key"]
    extra_args = []
    for section_id in get_video_sections(baseurl, token):
        url_ondeck = f"{baseurl}/library/sections/{section_id}/onDeck?X-Plex-Token={token}"
        try:
            r = requests.get(url_ondeck, timeout=5)
            r.raise_for_status()
            root = ElementTree.fromstring(r.content)
            for video in root.findall(".//Video"):
                part = video.find("Media/Part")
                if part is not None and part.attrib.get("key") == selected["part_key"]:
                    view_offset = int(video.attrib.get("viewOffset", "0"))
                    if view_offset > 0:
                        extra_args = [f"--start={view_offset/1000}"]
                    break
        except Exception as e:
            log_debug(f"Failed to fetch viewOffset for resume: {e}")
    log_debug(f"MPV extra_args for resume: {extra_args}")
    mpv_args = os.environ.get("MPV_CONFIG_OVERRIDE", "").split()
    subprocess.run(
        [
            "mpv",
            *mpv_args,
            "--force-window=yes",
            "--hwdec=vaapi",
            "--fullscreen",
            "--alang=eng",
            "--slang=eng",
            f"--title={selected['title']}",
            *extra_args,
            f"{url}?X-Plex-Token={token}",
        ]
    )

def menu_series():
    """Display the series menu and handle navigation through seasons and episodes."""
    while True:
        cur.execute("SELECT id, title FROM series ORDER BY title COLLATE NOCASE")
        series = cur.fetchall()
        title, _ = run_fzf(
            [t for _, t in series],
            "📺 Series: ",
            get_preview_script_series()
        )
        if not title:
            return

        s_id = dict((t, i) for i, t in series)[title]
        while True:
            cur.execute(
                "SELECT id, saison_index FROM saisons WHERE serie_id = ? ORDER BY saison_index",
                (s_id,),
            )
            seasons = cur.fetchall()
            label, _ = run_fzf(
                [f"Season {i}" for _, i in seasons],
                "📂 Season: "
            )
            if not label:
                break
            sa_id = dict((f"Season {i}", sid) for sid, i in seasons)[label]

            while True:
                cur.execute(
                    "SELECT episode_index, title, part_key, duration, summary, rating FROM episodes WHERE saison_id = ? ORDER BY episode_index",
                    (sa_id,),
                )
                episodes = cur.fetchall()
                e_map = [(f"{i:02d}. {t}", k, d, s, r) for i, t, k, d, s, r in episodes]

                choice, _ = run_fzf(
                    [label for label, *_ in e_map],
                    "🎞️ Episode: ",
                    get_preview_script_episode(sa_id)
                )
                if not choice:
                    break

                index = next(
                    (i for i, (label, *_) in enumerate(e_map) if label == choice), None
                )
                if index is None:
                    continue

                while 0 <= index < len(e_map):
                    label, part_key, *_ = e_map[index]
                    print(f"Playing: {label}")
                    launch_mpv(label, baseurl + part_key)

                    prev_label = (
                        f"⏮️ Previous: {e_map[index - 1][0]}" if index > 0 else None
                    )
                    next_label = (
                        f"⏭️ Next: {e_map[index + 1][0]}"
                        if index < len(e_map) - 1
                        else None
                    )

                    options = []
                    if next_label:
                        options.append(next_label)
                    if prev_label:
                        options.append(prev_label)

                    next_action = fzf_select(
                        "▶️ Choose action: ", options, default_first=True
                    )
                    if next_action and next_action.startswith("⏮️"):
                        index -= 1
                    elif next_action and next_action.startswith("⏭️"):
                        index += 1
                    else:
                        break

while True:
    choice = fzf_select("🎯 Choose: ", ["Continue Watching", "Movies", "Series"])
    if not choice:
        break
    if choice == "Continue Watching":
        menu_continue_watching()
    elif choice == "Movies":
        menu_films()
    elif choice == "Series":
        menu_series()
