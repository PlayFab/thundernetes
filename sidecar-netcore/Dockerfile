FROM mcr.microsoft.com/dotnet/aspnet:5.0-alpine AS base
WORKDIR /app
EXPOSE 56001

FROM mcr.microsoft.com/dotnet/sdk:5.0-alpine AS build
WORKDIR /src
COPY ["thundernetes-sidecar.csproj", ""]
RUN dotnet restore "./thundernetes-sidecar.csproj"
COPY . .
WORKDIR "/src/."
RUN dotnet build "thundernetes-sidecar.csproj" -c Release -o /app/build

FROM build AS publish
RUN dotnet publish "thundernetes-sidecar.csproj" -c Release -o /app/publish --runtime alpine-x64 \
    --self-contained true \
    /p:PublishTrimmed=true 

FROM base AS final
WORKDIR /app
COPY --from=publish /app/publish .
ENTRYPOINT ["dotnet", "thundernetes-sidecar.dll"]

#https://www.thorsten-hans.com/how-to-build-smaller-and-secure-docker-images-for-net5/