param ($repo)
$imageVersion = git rev-list HEAD --max-count=1 --abbrev-commit
$imageVersion = ":"+$imageVersion
$dockerfilesWin = Get-ChildItem -Include Dockerfile.win -Recurse
$dockerFolders = Split-Path -parent $dockerfilesWin
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker build -t $repo$imageName$imageVersion -f $dockerfilesWin[$i] .
}
for($i=0; $i -lt $dockerfilesWin.Length; $i++)
{
    $imageName = [System.IO.Path]::GetFileNameWithoutExtension($dockerFolders[$i])
    $imageName = "thundernetes-"+$imageName+"-win"
    docker push $repo$imageName$imageVersion
}
