using System;
using System.Diagnostics;
using System.Collections.Generic;
using Microsoft.Playfab.Gaming.GSDK.CSharp;

namespace openarena
{
    class Program
    {
        private static Process gameProcess;
        private static List<ConnectedPlayer> players = new List<ConnectedPlayer>();
        static void Main(string[] args)
        {
            // check here for the full guide on integrating with the GSDK 
            // https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/integrating-game-servers-with-gsdk

            LogMessage("OpenArena for Azure PlayFab Multiplayer Servers");
            
            LogMessage("Attempting to register GSDK callbacks");
            RegisterGSDKCallbacksAndStartGSDK();
            LogMessage("GSDK callback registration completed");
            
            LogMessage("Attempting to start game process");
            InitiateAndWaitForGameProcess();
            LogMessage("Game process has exited");
        }

        // starts main game process and wait for it to complete
        public static void InitiateAndWaitForGameProcess()
        {
             // here we're starting the script that initiates the game process
            gameProcess = StartProcess("/opt/startup.sh");
            // as part of wrapping the main game server executable,
            // we create event handlers to process the output from the game (standard output/standard error)
            // based on this output, we will activate the server and process connected players
            gameProcess.OutputDataReceived += DataReceived;
            gameProcess.ErrorDataReceived += DataReceived;
            // start reading output (stdout/stderr) from the game
            gameProcess.BeginOutputReadLine();
            gameProcess.BeginErrorReadLine();

            // wait till it exits or crashes
            gameProcess.WaitForExit();
        }

        // GSDK event handlers - we're setting them on the startup of the app
        public static void RegisterGSDKCallbacksAndStartGSDK()
        {
            // OnShutDown will be called when developer calls the ShutdDownMultiplayerServer API 
            // https://docs.microsoft.com/rest/api/playfab/multiplayer/multiplayerserver/shutdownmultiplayerserver?view=playfab-rest
            GameserverSDK.RegisterShutdownCallback(OnShutdown);
            // This callback will be called on every heartbeat to check if your game is healthy. So it should return quickly
            GameserverSDK.RegisterHealthCallback(IsHealthy);
            // this callback will be called to notify us that Azure will perform maintenance on the VM
            // you can see more details about Azure VM maintenance here https://docs.microsoft.com/en-gb/azure/virtual-machines/maintenance-and-updates?toc=/azure/virtual-machines/windows/toc.json&bc=/azure/virtual-machines/windows/breadcrumb/toc.json
            GameserverSDK.RegisterMaintenanceCallback(OnMaintenanceScheduled);
            
            // Call this while your game is initializing; it will start sending a heartbeat to our agent 
            // since our game server will transition to a standingBy state, we should call this when we're confident that our game server will not crash
            // here we're calling it just before we start our game server process - normally we would call it inside our game code
            GameserverSDK.Start();
        }

        // runs a Linux executable using Bash shell
        public static Process StartProcess(string cmd)
        {
            var escapedArgs = cmd.Replace("\"", "\\\"");
            var process = new Process()
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = "/bin/bash",
                    Arguments = $"-c \"{escapedArgs}\"",
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false,
                    CreateNoWindow = true,
                }
            };
            process.Start();
            return process;
        }

        // runs when we received data (stdout/stderr) from our game server process
        public static void DataReceived(object sender, DataReceivedEventArgs e)
        {
            Console.WriteLine(e.Data); // used for debug purposes only - you can use `docker logs <container_id> to see the stdout logs
            if(e.Data.Contains("Opening IP socket"))
            {
                // Call this when your game is done initializing and players can connect
                // Note: This is a blocking call, and will return when this game server is either allocated or terminated
                if(GameserverSDK.ReadyForPlayers())
                {
                    // After allocation, we can grab the session cookie from the config
                    IDictionary<string, string> activeConfig = GameserverSDK.getConfigSettings();

                    if (activeConfig.TryGetValue(GameserverSDK.SessionCookieKey, out string sessionCookie))
                    {
                        LogMessage($"The session cookie from the allocation call is: {sessionCookie}");
                    }
                }
                else
                {
                    // No allocation happened, the server is getting terminated (likely because there are too many already in standing by)
                    LogMessage("Server is getting terminated.");
                    gameProcess?.Kill(); // we still need to call WaitForExit https://docs.microsoft.com/dotnet/api/system.diagnostics.process.kill?view=netcore-3.1#remarks
                }
            }
            else if (e.Data.Contains("ClientBegin:")) // new player connected
            {
                players.Add(new ConnectedPlayer("gamer" + new Random().Next(0,21)));
                GameserverSDK.UpdateConnectedPlayers(players);
            }
            else if (e.Data.Contains("ClientDisconnect:")) // player disconnected
            {
                players.RemoveAt(new Random().Next(0, players.Count));
                GameserverSDK.UpdateConnectedPlayers(players);
                // some games may need to exit if player count is zero
            }
            else if (e.Data.Contains("AAS shutdown")) // game changes map
            {
                players.Clear();
                GameserverSDK.UpdateConnectedPlayers(players);
            }
        }

        static void OnShutdown()
        {
            LogMessage("Shutting down...");
            gameProcess?.Kill();
            Environment.Exit(0);
        }

        static bool IsHealthy()
        {
            // returns whether this game server process is healthy
            // here we're doing a simple check if our game wrapper is still alive
            return gameProcess != null;
        }

        static void OnMaintenanceScheduled(DateTimeOffset time)
        {
            LogMessage($"Maintenance Scheduled at: {time}");
        }

        private static void LogMessage(string message)
        {
            Console.WriteLine(message);
            // This will add your log line to the GSDK log file, alongside other information logged by the GSDK
            GameserverSDK.LogMessage(message);
        }
    }
}
