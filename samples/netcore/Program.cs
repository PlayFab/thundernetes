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
        const int HTTP_PORT = 80;
        private static DateTimeOffset _nextMaintenance = DateTimeOffset.MinValue;
        static void Main(string[] args)
        {
            GameserverSDK.Start(true);
            
            GameserverSDK.RegisterShutdownCallback(OnShutdown);
            GameserverSDK.RegisterHealthCallback(IsHealthy);
            GameserverSDK.RegisterMaintenanceCallback(OnMaintenanceScheduled);
            

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
                webBuilder.UseUrls($"http://*:{HTTP_PORT}");
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
