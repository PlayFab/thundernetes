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
            
            IDictionary<string, string> initialConfig = GameserverSDK.getConfigSettings();
            // Start the http server
            if (initialConfig?.ContainsKey(httpPortKey) == true)
            {
                httpPort = int.Parse(initialConfig[httpPortKey]);
            }
            else
            {
                Console.WriteLine("Cannot find gameport in GSDK Config Settings. Check your YAML definition");
                return;
            }

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
