# Piano Figure.

A grand piano, 88 keys (A0–C8), about 1.5 m wide and 1 m tall, lid open on a prop stick. Not in the preview at the moment.

Draft written from the existing code, for the user to correct.

## Animation Behavior.
1. `setKeyPressed(midi number, pressed boolean)`

    Presses or releases one key. Notes outside A0–C8 are ignored.

    Example:
    `piano.setKeyPressed(60, true)`
2. `releaseAll()`
3. `update(delta number)`, called every frame, eases the keys.
