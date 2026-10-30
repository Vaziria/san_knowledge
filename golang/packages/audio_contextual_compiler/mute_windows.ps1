# Mutes Sonic Pi's audio engine in Windows' volume mixer while ctxcompiler
# records a take, so the music is not heard; the take is recorded inside the
# engine, before the sound device, and is not changed. Only the engine
# processes started since -SinceUnixMs are muted, not a Sonic Pi already
# open. It mutes their audio sessions as they come until its standard input
# ends, then unmutes them: Windows remembers an app's mute.
param([string]$Name, [long]$SinceUnixMs)
$ErrorActionPreference = 'Stop'

Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Threading;

public static class CtxCompilerMute {
    [ComImport, Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")]
    class MMDeviceEnumerator {}

    [ComImport, Guid("A95664D2-9614-4F35-A746-DE8DB63617E6"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IMMDeviceEnumerator {
        [PreserveSig] int EnumAudioEndpoints(int dataFlow, int stateMask, out IMMDeviceCollection devices);
    }

    [ComImport, Guid("0BD7A1BE-7A1A-44DB-8397-CC5392387B5E"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IMMDeviceCollection {
        [PreserveSig] int GetCount(out int count);
        [PreserveSig] int Item(int index, out IMMDevice device);
    }

    [ComImport, Guid("D666063F-1587-4E43-81F1-B948E807363F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IMMDevice {
        [PreserveSig] int Activate(ref Guid iid, int clsCtx, IntPtr activationParams, [MarshalAs(UnmanagedType.IUnknown)] out object iface);
    }

    [ComImport, Guid("77AA99A0-1BD6-484F-8BC7-2C654C9A9B6F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IAudioSessionManager2 {
        [PreserveSig] int GetAudioSessionControl(IntPtr groupingParam, int flags, out IntPtr control);
        [PreserveSig] int GetSimpleAudioVolume(IntPtr groupingParam, int flags, out IntPtr volume);
        [PreserveSig] int GetSessionEnumerator(out IAudioSessionEnumerator sessions);
    }

    [ComImport, Guid("E2F5BB11-0570-40CA-ACDD-3AA01277DEE8"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IAudioSessionEnumerator {
        [PreserveSig] int GetCount(out int count);
        [PreserveSig] int GetSession(int index, out IAudioSessionControl2 session);
    }

    [ComImport, Guid("bfb7ff88-7239-4fc9-8fa2-07c950be9c6d"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IAudioSessionControl2 {
        [PreserveSig] int GetState(out int state);
        [PreserveSig] int GetDisplayName(out IntPtr name);
        [PreserveSig] int SetDisplayName(IntPtr name, IntPtr eventContext);
        [PreserveSig] int GetIconPath(out IntPtr path);
        [PreserveSig] int SetIconPath(IntPtr path, IntPtr eventContext);
        [PreserveSig] int GetGroupingParam(out Guid param);
        [PreserveSig] int SetGroupingParam(ref Guid param, IntPtr eventContext);
        [PreserveSig] int RegisterAudioSessionNotification(IntPtr client);
        [PreserveSig] int UnregisterAudioSessionNotification(IntPtr client);
        [PreserveSig] int GetSessionIdentifier(out IntPtr id);
        [PreserveSig] int GetSessionInstanceIdentifier(out IntPtr id);
        [PreserveSig] int GetProcessId(out int pid);
    }

    [ComImport, Guid("87CE5498-68D6-44E5-9215-6DA47EF883D8"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface ISimpleAudioVolume {
        [PreserveSig] int SetMasterVolume(float level, ref Guid eventContext);
        [PreserveSig] int GetMasterVolume(out float level);
        [PreserveSig] int SetMute([MarshalAs(UnmanagedType.Bool)] bool mute, ref Guid eventContext);
        [PreserveSig] int GetMute([MarshalAs(UnmanagedType.Bool)] out bool mute);
    }

    const int eRender = 0, DEVICE_STATE_ACTIVE = 1, CLSCTX_ALL = 23;

    // Engine sets the mute of every audio session, on every active output
    // device, of a process called name started since since, and returns
    // their process ids.
    static List<int> Engine(string name, DateTime since, bool mute) {
        var pids = new List<int>();
        var iid = typeof(IAudioSessionManager2).GUID;
        var ctx = Guid.Empty;
        var devices = (IMMDeviceEnumerator)new MMDeviceEnumerator();
        IMMDeviceCollection list;
        if (devices.EnumAudioEndpoints(eRender, DEVICE_STATE_ACTIVE, out list) != 0) return pids;
        int n;
        list.GetCount(out n);
        for (int i = 0; i < n; i++) {
            IMMDevice device;
            object o;
            if (list.Item(i, out device) != 0 || device.Activate(ref iid, CLSCTX_ALL, IntPtr.Zero, out o) != 0) continue;
            IAudioSessionEnumerator sessions;
            if (((IAudioSessionManager2)o).GetSessionEnumerator(out sessions) != 0) continue;
            int count;
            sessions.GetCount(out count);
            for (int j = 0; j < count; j++) {
                IAudioSessionControl2 session;
                int pid;
                if (sessions.GetSession(j, out session) != 0 || session.GetProcessId(out pid) != 0 || pid == 0) continue;
                try {
                    var p = Process.GetProcessById(pid);
                    if (!string.Equals(p.ProcessName, name, StringComparison.OrdinalIgnoreCase) || p.StartTime < since) continue;
                } catch (Exception) {
                    continue; // gone
                }
                ((ISimpleAudioVolume)session).SetMute(mute, ref ctx);
                pids.Add(pid);
            }
        }
        return pids;
    }

    public static void Run(string name, long sinceUnixMs) {
        var since = DateTimeOffset.FromUnixTimeMilliseconds(sinceUnixMs).LocalDateTime.AddSeconds(-2);
        var done = false;
        var reader = new Thread(() => {
            try { while (Console.In.ReadLine() != null) {} } catch (Exception) {}
            done = true;
        });
        reader.IsBackground = true;
        reader.Start();
        var told = new HashSet<int>();
        while (!done) {
            foreach (var pid in Engine(name, since, true)) {
                if (told.Add(pid)) Console.WriteLine("muted Sonic Pi's engine (process " + pid + ") in the volume mixer");
            }
            Thread.Sleep(100);
        }
        var back = Engine(name, since, false);
        if (told.Count > 0) Console.WriteLine("unmuted Sonic Pi's engine (" + back.Count + " sessions)");
    }
}
'@

[CtxCompilerMute]::Run($Name, $SinceUnixMs)
