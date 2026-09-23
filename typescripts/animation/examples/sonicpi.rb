use_bpm 72

# Every piano chord and melody note is also sent as OSC to the 3D preview's
# piano_play story. Sonic Pi sends OSC over UDP, which ngrok can't carry, so
# first run the relay on this computer (Node 18+); it passes each message on
# to the ngrok URL:
#
#   curl -o osc-relay.mjs https://07dd-180-244-133-36.ngrok-free.app/__osc/relay.mjs
#   node osc-relay.mjs https://07dd-180-244-133-36.ngrok-free.app
#
# and watch https://07dd-180-244-133-36.ngrok-free.app/?story=piano_play
# (the URL changes when ngrok restarts).
use_osc "localhost", 57121

# =========================
# LOFI WORK / STUDY
# =========================

# --- Piano chords ---
live_loop :piano do
  use_synth :piano
  
  progression = [
    chord(:c4, :major7),
    chord(:a3, :minor7),
    chord(:f3, :major7),
    chord(:g3, :dom7)
  ]
  names = ["Cmaj7", "Am7", "Fmaj7", "G7"] # the same chords, as the preview's piano reads them

  progression.each_with_index do |ch, i|
    osc "/piano/chord", names[i]
    play_chord ch,
      attack: 0.02,
      sustain: 1.8,
      release: 2.5,
      amp: 0.42,
      pan: Math.sin(i * 0.7) * 0.25
    
    sleep 2
  end
end


# --- Lazy piano melody ---
live_loop :melody do
  use_synth :piano
  
  melody = [
    :e5, nil, :g5, :b5,
    :a5, nil, :g5, :e5,
    :d5, nil, :e5, :g5,
    :e5, :d5, :c5, nil
  ]
  
  melody.each_with_index do |n, i|
    
    if n
      osc "/piano/note", note(n) # the MIDI number: 76 for :e5
      play n,
        release: 1.2,
        amp: 0.18,
        pan: Math.sin(i * 0.35) * 0.5
    end
    
    sleep 0.5
  end
end


# --- Soft bass ---
live_loop :bass do
  use_synth :fm
  
  roots = [:c2, :a2, :f2, :g2]
  
  roots.each do |n|
    play n,
      release: 1.5,
      amp: 0.16,
      depth: 1,
      divisor: 2,
      pan: -0.15
    
    sleep 2
  end
end


# --- Lo-fi drums ---
live_loop :drums do
  
  sample :bd_haus,
    amp: 0.35
  
  sleep 1
  
  sample :elec_hi_snare,
    amp: 0.13,
    pan: 0.1
  
  sleep 1
  
  sample :bd_haus,
    amp: 0.25
  
  sleep 1
  
  sample :elec_hi_snare,
    amp: 0.11,
    pan: -0.1
  
  sleep 1
end


# --- Quiet hi-hat ---
live_loop :hihat do
  sample :drum_cymbal_closed,
    amp: 0.06,
    pan: rrand(-0.3, 0.3)
  
  sleep 0.5
end


# --- Ambient texture ---
live_loop :texture do
  use_synth :hollow
  
  play :c5,
    attack: 3,
    sustain: 4,
    release: 4,
    amp: 0.035,
    pan: rrand(-0.7, 0.7)
  
  sleep 8
end