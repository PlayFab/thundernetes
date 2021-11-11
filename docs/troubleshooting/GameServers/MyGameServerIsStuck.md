## Help! What should we do if a GameServer gets stuck? 

We can advise to ```kubectl delete gs <Name>``` since this will take down both the GameServer instance, the corresponding Pod as well as the GameServerDetail CRD instance (in case the game server was allocated). 

:exclamation: Do not delete the Pod. It will be deleted automatically when the GameServer is deleted.
:exclamation: Do not manually override GameServer.status details, this will create issues during controller reconciliation. 