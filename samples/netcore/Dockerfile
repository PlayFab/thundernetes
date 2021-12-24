FROM mcr.microsoft.com/dotnet/aspnet:6.0-alpine AS base
WORKDIR /app
EXPOSE 56001

FROM mcr.microsoft.com/dotnet/sdk:6.0-alpine AS build
WORKDIR /src
COPY samples/netcore/netcore.csproj .
RUN dotnet restore "./netcore.csproj"
COPY samples/netcore/ .
WORKDIR "/src/."
RUN dotnet build "netcore.csproj" -c Release -o /app/build

FROM build AS publish
RUN dotnet publish "netcore.csproj" -c Release -o /app/publish
#--self-contained true \
#/p:PublishTrimmed=true \
#/p:PublishSingleFile=true

FROM base AS final
WORKDIR /app
COPY --from=publish /app/publish .
ENTRYPOINT ["dotnet", "netcore.dll"]
#ENTRYPOINT [ "./netcore" ]

#https://www.thorsten-hans.com/how-to-build-smaller-and-secure-docker-images-for-net5/