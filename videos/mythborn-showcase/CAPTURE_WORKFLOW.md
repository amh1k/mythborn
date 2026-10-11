# Reproducing the Mythborn demo captures

These captures use the real local Mythborn React app with a fixture API. The interface and bundled artwork come from the project; the civilization, agent responses, chronicle, beliefs, mysteries, and culture are fictional data in `demo-api.mjs`. No Supabase project or production account is used.

From the repository root, start these in separate terminals:

```sh
node videos/mythborn-showcase/demo-api.mjs
```

```sh
VITE_API_BASE_URL=http://127.0.0.1:6080 \
VITE_SUPABASE_URL= \
VITE_SUPABASE_PUBLISHABLE_KEY= \
node /home/amh1k/projects/mythborn/web/node_modules/vite/bin/vite.js \
  --config /home/amh1k/projects/mythborn/videos/mythborn-showcase/demo.vite.config.mjs \
  --configLoader runner
```

The local app is at `http://127.0.0.1:5187/worlds`. To reproduce the stateful page captures, run from `videos/mythborn-showcase`:

```sh
LD_LIBRARY_PATH=/tmp/mythborn-browser-libs/usr/lib/x86_64-linux-gnu \
node capture-interactions.mjs
```

The script opens the bundled headless Chrome, fills the founding form, selects the project watchtower image, opens the actual app tabs, and writes 1920×1080 screenshots to `capture-interactions/`. It requires the local Chrome libraries and the two local servers above. Stop both servers with Ctrl-C when finished.

The polished video uses `capture-polish/`. Reproduce these captures with:

```sh
MYTHBORN_CAPTURE_DIR=capture-polish MYTHBORN_CAPTURE_COMPLETE=1 \
LD_LIBRARY_PATH=/tmp/mythborn-browser-libs/usr/lib/x86_64-linux-gnu \
node capture-interactions.mjs
```

This waits for fonts, decoded images, and finite entry animations; preserves the approved harbor state through the app's carousel; and captures the remaining document below each scroll position. `measurements.json` records the resulting dimensions and actual control rectangles. The builder uses those control centers for the cursor and retains each capture's natural height, so lower reactions and rebuttals are available to the camera. The pointer's SVG hotspot stays on the measured center during its click pulse, with a continuous curved path between targets. The photograph is a hover stop; Begin discovery is the click.

The HyperFrames page captures in `capture-home-final/`, `capture-civ-final/`, and `capture-episode-final/` were made with the CLI `capture` command and `--skip-vision`; they use the same local app and fixture API. Earlier capture folders are retained as working history and are not the approved storyboard sources.

## Build, preview, and render the video

From this directory, regenerate the scene compositions after changing their source data:

```sh
node build-scenes.mjs
```

The builder keeps the motion source in one place and writes each scene's camera poses to `MOTION_PLAN.json`. Native WAAPI uses the global film clock: its delay includes the scene start exactly once. Scheduling a later scene from zero makes its animation finish before the scene appears.

The revised cut uses masked word entrances, unboxed explanation text, several camera poses per app view, and overlapping scene handoffs. Browser checks cover layout, runtime, and contrast. This CLI's static `keyframes` report does not detect native WAAPI; inspect consecutive browser snapshots and Studio playback to review that motion rather than treating an empty report as a pass.

Open the editable HyperFrames preview at `http://localhost:3002/#project/mythborn-showcase`:

```sh
npm run dev
```

Run the browser checks before rendering:

```sh
npm run check
```

After approving the preview, render a delivery MP4 with:

```sh
npm run render -- --quality delivery --output renders/mythborn-showcase.mp4
```
