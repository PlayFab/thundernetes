# this script will build the docker images for windows from the local files
# and then upload them to a registry so you can deploy them

param ($registry, $version)
# registry: this parameter is the url to your container registry, it should end with a / symbol,
#           you also need to login to your registry before running the script
# version: this parameter should not be set for local development, so it generates a version using
#          the current commit, but you can set the value if you're trying something

if (-Not $PSBoundParameters.ContainsKey("version"))
{
    $version = git rev-list HEAD --max-count=1 --abbrev-commit
}
if (-Not $PSBoundParameters.ContainsKey("registry"))
{
    $registry = "ghcr.io/playfab/"
}
write-host "version:"$version
write-host "registry:"$registry
# get all windows dockerfiles, it searches all files named Dockerfile.win 
$version = ":"+$version
$dockerfilesWin = Get-ChildItem -Include Dockerfile.win -Recurse
$dockerFolders = Split-Path -parent $dockerfilesWin
# build images
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker build -t $registry$imageName$version -f $dockerfilesWin[$i] .
}
# push images
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker push $registry$imageName$version
}
