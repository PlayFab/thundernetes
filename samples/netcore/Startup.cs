using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Playfab.Gaming.GSDK.CSharp;

namespace netcore
{
    public class Startup
    {
        public Startup(IConfiguration configuration)
        {
            Configuration = configuration;
        }

        public IConfiguration Configuration { get; }

        // This method gets called by the runtime. Use this method to add services to the container.
        public void ConfigureServices(IServiceCollection services)
        {
            services.AddControllers();
        }

        // This method gets called by the runtime. Use this method to configure the HTTP request pipeline.
        public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
        {
            if (env.IsDevelopment())
            {
                app.UseDeveloperExceptionPage();
            }

            app.UseRouting();

            app.UseAuthorization();

            Task.Run(()=>ReadyForPlayersTask());
            
            app.UseEndpoints(endpoints =>
            {
                endpoints.MapControllers();
            });
        }

        private async static Task ReadyForPlayersTask()
        {
            Utils.LogMessage("Before ReadyForPlayers");
            GameserverSDK.ReadyForPlayers();
            Utils.LogMessage("After ReadyForPlayers");
            PrintGSDKInfo();
            var initialPlayers = GameserverSDK.GetInitialPlayers();
            Console.WriteLine("Initial Players: " + String.Join("-", initialPlayers));
            await Task.Delay(TimeSpan.FromSeconds(1));
            GameserverSDK.UpdateConnectedPlayers(new List<ConnectedPlayer>() 
            {
                new ConnectedPlayer("Amie"), 
                new ConnectedPlayer("Ken"),
                new ConnectedPlayer("Dimitris")
            });
        }

        private static void PrintGSDKInfo()
        {   
            var config = GameserverSDK.getConfigSettings();
            Console.WriteLine("Start - printing config settings");
            foreach(var item in config)
            {
                Console.WriteLine($"    Config with key {item.Key} has value {item.Value}");
            }
            Console.WriteLine("End - printing config settings");

        }        
    }

    public static class Utils
    {
        public static void LogMessage(string message, bool enableGSDKLogging = true)
        {
            Console.WriteLine(message);
            if (enableGSDKLogging)
            {
                GameserverSDK.LogMessage(message);
            }
        }
    }
}