# Compiled by ctxcompiler from folk_song.md (graph: folk_song.graph.json).

# FolkSong - a slow, relaxing morning coffee-shop folk song: a gamelan meets a guitar.
# Scale "pelog": Sonic Pi has no pelog scale, so the notes are computed here as
# fractional MIDI note numbers. The guitar is "tuned" to the same pelog pitches (tonic E).
# Total: Intro 1 min (64 bpm, 64 beats) + Climax 4 min (2 min at 96 bpm + 2 min at 112 bpm) = 5 min.
use_bpm 64
use_random_seed 1964

# 7-tone pelog degree -> fractional MIDI note
# (approximate Javanese tuning in cents: 0 120 270 540 670 785 950)
define :pelog_pitch do |deg, base|
  offs = [0.0, 1.2, 2.7, 5.4, 6.7, 7.85, 9.5]
  base + 12 * deg.div(7) + offs[deg % 7]
end

# pelog bem (5-tone subset, degrees 1 2 3 5 6) index -> fractional MIDI note
define :bem_pitch do |idx, base|
  sub = [0, 1, 2, 4, 5]
  pelog_pitch(sub[idx % 5] + 7 * idx.div(5), base)
end

# Gong ageng: Sonic Pi has no gong. Approximated with a low sine, a slightly detuned sine
# (the slow beating of a gong), a sub-octave sine and an inharmonic FM shimmer, long decay.
define :gong_stroke do |base, level, decay_beats|
  synth :sine, note: base, attack: 0.01, sustain: 0.3, release: decay_beats, amp: 0.7 * level
  synth :sine, note: base + 0.15, attack: 0.05, release: decay_beats * 0.9, amp: 0.4 * level
  synth :sine, note: base - 12, attack: 0.03, release: decay_beats * 0.7, amp: 0.5 * level
  synth :fm, note: base + 12, divisor: 1.41, depth: 1.2, attack: 0.005, release: decay_beats * 0.35, amp: 0.2 * level
end

# Bonang: Sonic Pi has no bonang (kettle gongs). Approximated with :pretty_bell, short release.
# Imbal (interlocking): voice A on the beats (left), voice B off the beats (right),
# both land on the gatra's target note on its last beat.
define :bonang_cycle do |bal, base, level, interlock|
  bal.each do |t|
    pot_notes = [t + 1, t + 3, t + 2, t + 3, t + 1, t + 2, t, t + 1]
    pot_notes.each_with_index do |n, i|
      if i.even?
        synth :pretty_bell, note: bem_pitch(n, base), amp: level * rrand(0.9, 1.0), pan: -0.4, attack: 0.002, release: 0.7
      elsif interlock
        synth :pretty_bell, note: bem_pitch(n, base), amp: level * 0.8 * rrand(0.9, 1.0), pan: 0.4, attack: 0.002, release: 0.6
      end
      sleep 0.5
    end
  end
end

# Kendang: Sonic Pi has no kendang. Approximated with tabla samples:
# :b = dhah (deep), :d = dhung (open low), :t = tak (slap), :k = ket (closed)
define :kendang_hit do |stroke, level|
  case stroke
  when :b then sample :tabla_dhec, amp: 0.9 * level
  when :d then sample :tabla_tun1, amp: 0.7 * level
  when :t then sample :tabla_na, amp: 0.6 * level
  when :k then sample :tabla_ke1, amp: 0.5 * level
  end
end

define :kendang_pattern do |strokes, step, level|
  strokes.each do |s|
    kendang_hit(s, level * rrand(0.85, 1.0)) if s
    sleep step
  end
end

# node: intro
define :intro do
  use_bpm 64
  # 64 beats = 1 min, four 16-beat cycles

  # node: intro.gong
  # one stroke at the end (last beat) of each 16-beat cycle
  in_thread do
    with_fx :reverb, room: 0.9, mix: 0.5 do
      4.times do |c|
        sleep 15
        gong_stroke(40, 0.6 + 0.1 * c, 10)
        sleep 1
      end
    end
  end

  # node: intro.guitar
  # soft fingerpicked guitar (:pluck, Karplus-Strong string), tuned to pelog bem on E
  in_thread do
    groot = 52
    bass_line = [0, 3, 0, 2, 3, 4, 2, 0, 0, 2, 4, 3, 2, 1, 3, 0]
    guitar_tune = [
      [5, 2], [6, 1], [7, 1], [8, 3], [7, 1], [6, 2], [5, 2], [7, 4],
      [8, 2], [9, 2], [8, 1], [7, 1], [6, 2], [7, 3], [6, 1], [5, 4],
      [5, 1], [6, 1], [7, 2], [8, 2], [9, 2], [10, 3], [9, 1], [8, 4],
      [7, 2], [6, 2], [5, 2], [4, 2], [3, 3], [4, 1], [5, 4]
    ]
    with_fx :reverb, room: 0.8, mix: 0.4 do
      with_fx :lpf, cutoff: 100 do
        # fingerpicked accompaniment: thumb on the beats, fingers between (16 bars x 4 beats)
        in_thread do
          bass_line.each do |b|
            finger_notes = [b - 5, b + 2, b, b + 3, b - 3, b + 2, b, b + 1]
            finger_notes.each_with_index do |n, i|
              synth :pluck, note: bem_pitch(n, groot), amp: (i.even? ? 0.4 : 0.25) * rrand(0.85, 1.0), pan: (i.even? ? -0.2 : 0.15), coef: 0.4, release: 1.5
              sleep 0.5
            end
          end
        end
        # the melody on top (4 phrases x 16 beats)
        guitar_tune.each do |m|
          synth :pluck, note: bem_pitch(m[0], groot), amp: 0.55 * rrand(0.9, 1.0), pan: 0.1, coef: 0.3, release: 2
          sleep m[1]
        end
      end
    end
  end

  sleep 64
end

# node: climax
define :climax do
  use_bpm 96
  fast_bpm = 112
  # 4 min: 12 cycles x 16 beats at 96 bpm (2 min), then the kendang signals the speed-up
  # (cue :speed_up = Kendang.signal -> all.speedUp) and 14 cycles x 16 beats at 112 bpm (2 min).
  # Section thread: 384 beats at 96 bpm = 240 s.

  # node: climax.kendang
  # leads the tempo; plays the speed-up signal fill and then cues :speed_up for everyone
  in_thread do
    gatra_a = [:d, nil, :k, :t, nil, :d, :t, nil]
    gatra_b = [:d, :k, :t, :k, :d, :t, :b, nil]
    gatra_c = [:d, :k, :t, :k, :d, :d, :t, :k]
    signal_fill = [:t, :k, :t, :k, :d, :t, :k, :t, :d, :d, :t, :k, :b, nil, :b, nil]
    ending = [:t, :k, :d, nil, :t, :t, :b, nil]
    with_fx :reverb, room: 0.5, mix: 0.25 do
      # phase 1: 96 bpm
      12.times do |c|
        4.times do |g|
          if c == 11 && g == 3
            kendang_pattern(signal_fill, 0.25, 1.0)
          elsif g == 3
            kendang_pattern(gatra_b, 0.5, 0.85)
          else
            kendang_pattern(gatra_a, 0.5, 0.8)
          end
        end
      end
      # signal: speed up
      cue :speed_up
      use_bpm fast_bpm
      # phase 2: 112 bpm
      14.times do |c|
        4.times do |g|
          if c == 13 && g == 3
            kendang_pattern(ending, 0.5, 1.0)
          elsif g == 3
            kendang_pattern(gatra_b, 0.5, 1.0)
          elsif g.odd?
            kendang_pattern(gatra_c, 0.5, 0.9)
          else
            kendang_pattern(gatra_a, 0.5, 0.9)
          end
        end
      end
    end
  end

  # node: climax.gong
  # marks each cycle; follows climax.kendang: same 16-beat cycle, and switches to the
  # kendang's faster tempo when it syncs on the kendang's :speed_up cue
  in_thread do
    in_thread do
      sync :speed_up
      use_bpm fast_bpm
      with_fx :reverb, room: 0.9, mix: 0.45 do
        14.times do |c|
          sleep 15
          gong_stroke(40, (c == 13 ? 1.0 : 0.85), (c == 13 ? 16 : 10))
          sleep 1
        end
      end
    end
    with_fx :reverb, room: 0.9, mix: 0.45 do
      12.times do
        sleep 15
        gong_stroke(40, 0.8, 10)
        sleep 1
      end
    end
  end

  # node: climax.bonang
  # interlocking (imbal) pattern over a balungan; follows climax.kendang: speeds up on :speed_up
  in_thread do
    broot = 64
    bal_a = [2, 1, 3, 0]
    bal_b = [3, 4, 2, 0]
    bal_c = [4, 3, 1, 0]
    bal_d = [1, 2, 4, 0]
    in_thread do
      sync :speed_up
      use_bpm fast_bpm
      with_fx :reverb, room: 0.7, mix: 0.35 do
        14.times do |c|
          bonang_cycle((c.even? ? bal_c : bal_d), broot, 0.28, true)
        end
      end
    end
    with_fx :reverb, room: 0.7, mix: 0.35 do
      12.times do |c|
        # the first two cycles only voice A, then the interlocking starts
        bonang_cycle((c.even? ? bal_a : bal_b), broot, 0.22, c >= 2)
      end
    end
  end

  sleep 384
end

intro
climax
