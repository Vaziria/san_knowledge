import { lakeEnvironment, type Environment } from '../../previews';
import type { Story } from '../../stories';
import type { Theme } from '../../theme';
import { CLEAR, DOCK, LOGS, Meeting } from './Meeting';

// Lake meeting (lake_meeting.md): the viewers of the YouTube live stream meet
// at the lake. The dev server reads the stream's live chat with san_youtube
// and keeps who is there (chat-bridge.ts, meeting.ts); the Story tab picks
// the channel and can send made-up messages to try it without YouTube.
//
// - The host is the Bear, which starts on the landing and roams the lake
//   like the others; the channel owner's messages and commands go to it.
// - A viewer's first message brings their animal, of a random kind: a cat,
//   wolf, deer, bird, fox, snake, penguin, frog or lion; !cat, !wolf and the
//   other kinds switch it. Its messages are its speech bubbles, each headed by the
//   viewer's name, and !jump, !walk, !run, !stop and !flap (a penguin's) are
//   commands.
// - At most 12 viewers' animals; a new one past that destroys the one that
//   joined first (FIFO), and one whose viewer is quiet for 10 minutes walks
//   away. Bots of random kinds keep the meeting at seven or more, giving up
//   their places to viewers.
// - The fish leaps out of the lake for a Super Chat, a Super Sticker or a new
//   member, higher for a bigger Super Chat.
// - The camera follows each comment and command in turn, so none is missed
//   (Meeting.ts); when the chat is quiet it goes back to the host, wherever
//   it is (HostView.ts), and after 30 s it follows one animal after another
//   from a few angles (Follow.ts).
//
// It plays on the lake, built with a clearing round the landing wide enough
// for the meeting (level, mown and free of stones and meadow plants), a dock
// out from the far bank that the animals walk out on (DOCK), and logs
// floating in the water (LOGS).
function meetingLake(theme: Theme): Environment {
  return lakeEnvironment(theme, { clear: CLEAR, dock: DOCK, logs: LOGS });
}

export const lakeMeeting: Story = {
  environment: meetingLake,
  create: (theme, environment) => {
    const meeting = new Meeting(theme, environment);
    return {
      figure: meeting,
      camera: Meeting.WIDE.camera.toArray(),
      target: Meeting.WIDE.target.toArray(),
      bounds: meeting.bounds,
      followShadow: true,
      update: (_elapsed, delta) => meeting.update(delta),
      dispose: () => meeting.dispose(),
      shot: () => meeting.shot(),
    };
  },
};
