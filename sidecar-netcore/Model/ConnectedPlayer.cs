namespace thundernetes_sidecar.Model
{
    public class ConnectedPlayer
    {
        public string PlayerId { get; set; }

        public ConnectedPlayer(string playerid)
        {
            this.PlayerId = playerid;
        }
    }
}