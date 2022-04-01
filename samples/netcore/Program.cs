using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Playfab.Gaming.GSDK.CSharp;

namespace netcore
{
    class Program
    {
        private static DateTimeOffset _nextMaintenance = DateTimeOffset.MinValue;
        private static int httpPort;
        private static string httpPortKey = "gameport";
        static void Main(string[] args)
        {
            GameserverSDK.Start(true);
            
            GameserverSDK.RegisterShutdownCallback(OnShutdown);
            GameserverSDK.RegisterHealthCallback(IsHealthy);
            GameserverSDK.RegisterMaintenanceCallback(OnMaintenanceScheduled);
            
            var gameServerConnectionInfo = GameserverSDK.GetGameServerConnectionInfo();
            var portInfo = gameServerConnectionInfo.GamePortsConfiguration.Where(x=>x.Name == httpPortKey);
            if(portInfo.Count() == 0)
            {
                throw new Exception("No port info found for " + httpPortKey);
            }
            httpPort = portInfo.Single().ServerListeningPort;

            Console.WriteLine($"Welcome to fake game server!");
            if(args.Length > 0)
            {
                foreach(string arg in args)
                {
                    Console.WriteLine($"Argument: {arg}");
                } 
            }
            CreateHostBuilder(args).Build().Run();
        }

        public static IHostBuilder CreateHostBuilder(string[] args) =>
        Host.CreateDefaultBuilder(args)
            .ConfigureWebHostDefaults(webBuilder =>
            {
                webBuilder.UseStartup<Startup>();
                webBuilder.UseUrls($"http://*:{httpPort}");
            });


        static void OnShutdown()
        {
            Utils.LogMessage("Shutting down...");
        }

        static bool IsHealthy()
        {
            // Should return whether this game server is healthy
            return true;
        }

        static void OnMaintenanceScheduled(DateTimeOffset time)
        {
            Utils.LogMessage($"Maintenance Scheduled at: {time}");
            _nextMaintenance = time;
        }
    }
}
