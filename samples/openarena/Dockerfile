FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /source

# copy csproj and restore as distinct layers
COPY samples/openarena/*.csproj .
RUN dotnet restore

# copy and publish app and libraries
COPY samples/openarena/ .
RUN dotnet publish -c release -o /app -r linux-x64

# final stage/image
FROM fgracia/openarena:latest
WORKDIR /app
COPY --from=build /app .
EXPOSE 27960/udp
CMD ["./openarena"]