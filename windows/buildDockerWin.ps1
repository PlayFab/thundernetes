param ($registry, $version)

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
$version = ":"+$version
$dockerfilesWin = Get-ChildItem -Include Dockerfile.win -Recurse
$dockerFolders = Split-Path -parent $dockerfilesWin
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker build -t $registry$imageName$version -f $dockerfilesWin[$i] .
}
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker push $registry$imageName$version
}
