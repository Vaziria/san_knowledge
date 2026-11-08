# MovieMov examples

Workflows for a site made from web_movie (`php/apps/web_movie`: MovieMov at
https://moviemov.id, or any other deploy of it), through the MovieMov steps
(Web › MovieMov) and a MovieMov connection (the user, 2026-11-08: "In tower
can we have node to upload to this web that deployed with web_movie ?").
They were built in a tower and exported (ExportWorkflows), so they are
exactly the tower's format. See the tower's README (Workflows › The MovieMov
steps, and Connections › MovieMov) for the details.

| File | Workflow | What it does |
| --- | --- | --- |
| `moviemov-upload-draft.workflow.json` | MovieMov - Upload Draft | Manual → Video Thumbnail → MovieMov Upload Movie: takes a 16:9 frame of the video at 2 s, 1280 wide (Video Thumbnail's Aspect 16:9, Width 1280), and uploads the movie with it as a **draft** (hidden until you publish it), with the title and the player from the run's Payload. |

## Import and switch on

1. Make a MovieMov connection (Connections page › MovieMov): Base URI
   `https://moviemov.id/api` (the default), Username the admin's email,
   Password the admin's password. Test says which admin answered. Name it
   `MovieMov` and the import picks it by name; otherwise it takes the only
   MovieMov connection, or the import notes say to pick one in MovieMov
   Upload Movie.
2. Workflows page › Import, and pick the file. Or through the API:

   ```sh
   python -c "import json,sys; print(json.dumps({'json': open(sys.argv[1], encoding='utf-8').read()}))" moviemov-upload-draft.workflow.json > body.json
   curl http://127.0.0.1:7477/san.tower.v1.WorkflowService/ImportWorkflows -H 'Content-Type: application/json' -d @body.json
   ```

The workflow starts with a Manual trigger, so it needs no switching on: it
runs only from the editor's Run (or RunWorkflow).

## Run it

In the editor, Run, with the Payload:

```json
{
  "video": "D:/videos/night-train.mp4",
  "title": "Night Train",
  "embed_url": "<iframe src=\"https://vidara.to/e/XXXX\" width=\"640\" height=\"360\" frameborder=\"0\" allowfullscreen></iframe>"
}
```

- `video` is the video file on this PC; the thumbnail is saved beside it
  (`night-train_thumb_2s_16x9.jpg`).
- `embed_url` is the player's https link or its whole `<iframe>` code
  (Vidara's embed code works as it is).
- Categories, Tags, Synopsis, Release Year, Duration and Backup Embed URLs
  can be typed in the MovieMov Upload Movie step (Categories as `Drama,
  Thriller`), or linked from other steps.

MovieMov Upload Movie's Movie, URL and Slug give the movie: link Slug to a
MovieMov Update Movie with Status Published (or Scheduled with Publish At,
in WIB) to put it live, or check it on the site's admin first.
