FROM mcr.microsoft.com/dotnet/core/sdk:3.1 AS build
WORKDIR /source

# copy csproj and restore as distinct layers
COPY *.csproj .
RUN dotnet restore

# copy and publish app and libraries
COPY . .
RUN dotnet publish -c release -o /app -r linux-x64

# final stage/image
FROM fgracia/openarena:latest
WORKDIR /app
COPY --from=build /app .
EXPOSE 27960/udp
CMD ["./openarena"]