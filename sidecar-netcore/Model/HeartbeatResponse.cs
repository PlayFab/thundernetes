namespace thundernetes_sidecar.Model
{
    using Newtonsoft.Json;
    using Newtonsoft.Json.Converters;

    internal class HeartbeatResponse
    {
        [JsonProperty(PropertyName = "sessionConfig")]
        public SessionConfig SessionConfig { get; set; }

        [JsonProperty(PropertyName = "nextScheduledMaintenanceUtc")]
        public string NextScheduledMaintenanceUtc { get; set; }

        [JsonProperty(PropertyName = "operation", ItemConverterType = typeof(StringEnumConverter))]
        public GameOperation Operation { get; set; }
    }
    
    public enum GameState
    {
        Invalid,
        Initializing,
        StandingBy,
        Active,
        Terminating,
        Terminated,
        Quarentined
    }

    public enum GameOperation
    {
        Invalid,
        Continue,
        Active,
        Terminate
    }

}