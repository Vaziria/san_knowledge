# Folk song

An example prompt spec context: a slow, relaxing folk song that mixes a
gamelan with a guitar, for a coffee shop in the morning.

```
class Section {              // every section has the same fields
    length
    tempo                    // from the song unless set
    mood
    instruments
}

class FolkSong {
    scale    = "pelog"       // the guitar is tuned to match
    tempo    = "slow"
    mood     = "relaxing"
    duration = 5min

    flow = [
        Intro(length = 1min),
        Climax(length = 4min)
    ]
}

class Intro extends Section {
    mood = "spacious"
    instruments = [
        Gong(role = "one stroke at the end of each cycle"),
        Guitar(role = "soft fingerpicked melody")
    ]
}

class Climax extends Section {
    tempo = "medium"         // overrides the song's "slow"
    instruments = [
        Gong(role = "marks each cycle"),
        Bonang(role = "interlocking pattern"),
        Kendang(role = "leads the tempo")
    ]
    on Kendang.signal -> all.speedUp
}
```
